package client

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"
	"bytes"
	"encoding/gob"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// NetworkClient lida com a comunicação com o Servidor FortressVision
type NetworkClient struct {
	conn      *websocket.Conn
	url       string
	store     *mapdata.MapDataStore
	connected bool
	mu        sync.RWMutex

	// Callbacks para o App
	OnMapChunk    func(origin util.DFCoord)
	OnStatus      func(msg string, dfConnected bool)
	OnWorldStatus func(status *fvnet.WorldStatus)
	OnTiletypes   func(list *dfproto.TiletypeList)
	OnMaterials   func(list *dfproto.MaterialList)
}

func NewNetworkClient(url string, store *mapdata.MapDataStore) *NetworkClient {
	return &NetworkClient{
		url:   url,
		store: store,
	}
}

func (c *NetworkClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		log.Printf("[Network] Tentativa de conexão %d/%d em %s...", i+1, maxRetries, c.url)
		c.conn, _, err = dialer.Dial(c.url, nil)
		if err == nil {
			break
		}
		log.Printf("[Network] Servidor ainda não está pronto: %v. Aguardando...", err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Printf("[Network] ERRO CRÍTICO após %d tentativas: %v", maxRetries, err)
		return err
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

func (c *NetworkClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *NetworkClient) RequestRegion(center util.DFCoord, radius int32) {
	req := &fvnet.ClientRequestRegion{
		CenterX: center.X,
		CenterY: center.Y,
		CenterZ: center.Z,
		Radius:  radius,
	}
	c.Send(fvnet.Envelope_CLIENT_REQUEST_REGION, req)
}

func (c *NetworkClient) Send(msgType fvnet.Envelope_Type, msg proto.Message) {
	if !c.IsConnected() {
		return
	}

	var payload []byte
	var err error
	if msg != nil {
		payload, err = proto.Marshal(msg)
		if err != nil {
			log.Printf("[Network] Erro ao serializar payload: %v", err)
			return
		}
	}

	env := &fvnet.Envelope{
		Type:    msgType,
		Payload: payload,
	}

	data, err := proto.Marshal(env)
	if err != nil {
		log.Printf("[Network] Erro ao serializar envelope: %v", err)
		return
	}

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.BinaryMessage, data)
	c.mu.Unlock()

	if err != nil {
		log.Printf("[Network] Erro ao enviar mensagem: %v", err)
		c.connected = false
	}
}

func (c *NetworkClient) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[Network] Conexão perdida: %v", err)
			break
		}

		var env fvnet.Envelope
		if err := proto.Unmarshal(message, &env); err != nil {
			log.Printf("[Network] Erro ao desempacotar envelope: %v", err)
			continue
		}

		c.handleMessage(&env)
	}
}

func (c *NetworkClient) handleMessage(env *fvnet.Envelope) {
	switch env.Type {
	case fvnet.Envelope_SERVER_STATUS:
		var status fvnet.ServerStatus
		if err := proto.Unmarshal(env.Payload, &status); err == nil {
			if c.OnStatus != nil {
				c.OnStatus(status.Message, status.DfConnected)
			}
		}
	case fvnet.Envelope_MAP_CHUNK:
		var chunkMsg fvnet.MapChunkMessage
		if err := proto.Unmarshal(env.Payload, &chunkMsg); err == nil {
			log.Printf("[Network] Chunk recebido: Z=%d (%d, %d)", chunkMsg.ChunkZ, chunkMsg.ChunkX, chunkMsg.ChunkY)
			c.processChunk(&chunkMsg)
		}
	case fvnet.Envelope_WORLD_STATUS:
		var worldStatus fvnet.WorldStatus
		if err := proto.Unmarshal(env.Payload, &worldStatus); err == nil {
			if c.OnWorldStatus != nil {
				c.OnWorldStatus(&worldStatus)
			}
		}
	case fvnet.Envelope_TILETYPE_LIST:
		var list dfproto.TiletypeList
		if err := list.Unmarshal(env.Payload); err == nil {
			log.Printf("[Network] Recebidos %d tiletypes do servidor", len(list.TiletypeList))
			if c.OnTiletypes != nil {
				c.OnTiletypes(&list)
			}
		}
	case fvnet.Envelope_MATERIAL_LIST:
		var list dfproto.MaterialList
		if err := list.Unmarshal(env.Payload); err == nil {
			log.Printf("[Network] Recebidos %d materiais do servidor", len(list.MaterialList))
			if c.OnMaterials != nil {
				c.OnMaterials(&list)
			}
		}
	case fvnet.Envelope_PONG:
		// Ping/Pong handled
	case fvnet.Envelope_VEGETATION_UPDATE:
		var vegMsg fvnet.VegetationUpdateMessage
		if err := vegMsg.Unmarshal(env.Payload); err == nil {
			c.processVegetation(&vegMsg)
		}
	}
}

func (c *NetworkClient) processVegetation(msg *fvnet.VegetationUpdateMessage) {
	origin := util.DFCoord{X: msg.ChunkX, Y: msg.ChunkY, Z: msg.ChunkZ}

	var plants []dfproto.PlantDetail
	for _, p := range msg.Plants {
		plants = append(plants, dfproto.PlantDetail{
			Pos:      dfproto.Coord{X: p.X, Y: p.Y, Z: 0},
			Material: dfproto.MatPair{MatType: p.MatType, MatIndex: p.MatIndex},
		})
	}

	c.store.StorePlants(msg.ChunkX, msg.ChunkY, msg.ChunkZ, plants)

	// Invalida o mesh para re-renderizar o crescimento
	if c.OnMapChunk != nil {
		c.OnMapChunk(origin)
	}
}

func (c *NetworkClient) processChunk(msg *fvnet.MapChunkMessage) {
	origin := util.DFCoord{X: msg.ChunkX, Y: msg.ChunkY, Z: msg.ChunkZ}

	// Se VoxelData for nil, é um chunk de "Ar" (vazio)
	if msg.VoxelData == nil {
		c.store.Mu.Lock()
		delete(c.store.Chunks, origin) // Garante que não há lixo
		c.store.Mu.Unlock()
		if c.OnMapChunk != nil {
			c.OnMapChunk(origin)
		}
		return
	}

	// Decodificar Tiles via GOB
	var tiles [16][16]*mapdata.Tile
	dec := gob.NewDecoder(bytes.NewReader(msg.VoxelData))
	if err := dec.Decode(&tiles); err != nil {
		log.Printf("[Network] Erro ao decodificar tiles do chunk %v: %v", origin, err)
		return
	}

	// Inserir no MapStore local
	c.store.Mu.Lock()
	chunk := &mapdata.Chunk{
		Origin: origin,
		Tiles:  tiles,
		MTime:  time.Now().UnixNano(), // Nova versão local
	}

	// Re-conecta os tiles ao store local p/ consultas
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if t := chunk.Tiles[x][y]; t != nil {
				t.SetStore(c.store)
			}
		}
	}

	c.store.Chunks[origin] = chunk
	c.store.Mu.Unlock()

	if c.OnMapChunk != nil {
		c.OnMapChunk(origin)
	}
}
