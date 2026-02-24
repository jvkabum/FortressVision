package client

import (
	"FortressVision/shared/mapdata"
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
}

func NewNetworkClient(url string, store *mapdata.MapDataStore) *NetworkClient {
	return &NetworkClient{
		url:   url,
		store: store,
	}
}

func (c *NetworkClient) Connect() error {
	log.Printf("[Network] Conectando ao servidor em %s...", c.url)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
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
			c.processChunk(&chunkMsg)
		}
	case fvnet.Envelope_WORLD_STATUS:
		var worldStatus fvnet.WorldStatus
		if err := proto.Unmarshal(env.Payload, &worldStatus); err == nil {
			if c.OnWorldStatus != nil {
				c.OnWorldStatus(&worldStatus)
			}
		}
	case fvnet.Envelope_PONG:
		// Ping/Pong handled
	}
}

func (c *NetworkClient) processChunk(msg *fvnet.MapChunkMessage) {
	origin := util.DFCoord{X: msg.ChunkX, Y: msg.ChunkY, Z: msg.ChunkZ}

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
				// tile.container é privado, mas mapdata pode acessá-lo se estiver no mesmo pacote.
				// Como estamos em client, precisamos de um helper no mapdata ou tornar o container público/usar um setter.
				// Por enquanto, vamos assumir que o cliente cuidará da renderização sem precisar do container.
			}
		}
	}

	c.store.Chunks[origin] = chunk
	c.store.Mu.Unlock()

	if c.OnMapChunk != nil {
		c.OnMapChunk(origin)
	}
}
