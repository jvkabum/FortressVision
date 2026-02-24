package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"FortressVision/servidor/internal/dfhack"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Hub gerencia as conexões WebSocket ativas
type Hub struct {
	clients    map[*websocket.Conn]*sync.Mutex
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]*sync.Mutex),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = &sync.Mutex{}
			h.mu.Unlock()
			log.Printf("Cliente registrado: %s", client.RemoteAddr())
		case client := <-h.unregister:
			h.mu.Lock()
			if lock, ok := h.clients[client]; ok {
				lock.Lock() // Garante que nenhuma escrita ocorra durante o fechamento
				delete(h.clients, client)
				client.Close()
				lock.Unlock()
				log.Printf("Cliente desregistrado: %s", client.RemoteAddr())
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			// Criamos uma cópia local para evitar segurar o lock do hub durante as escritas
			activeClients := make(map[*websocket.Conn]*sync.Mutex)
			for c, l := range h.clients {
				activeClients[c] = l
			}
			h.mu.Unlock()

			for client, lock := range activeClients {
				lock.Lock()
				err := client.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Printf("Erro ao enviar para cliente: %v", err)
					client.Close()
					h.mu.Lock()
					delete(h.clients, client)
					h.mu.Unlock()
				}
				lock.Unlock()
			}
		}
	}
}

// WriteSafe garante que apenas uma goroutine escreva no WebSocket por vez
func (h *Hub) WriteSafe(conn *websocket.Conn, messageType int, data []byte) error {
	h.mu.Lock()
	lock, ok := h.clients[conn]
	h.mu.Unlock()

	if !ok {
		return fmt.Errorf("cliente não encontrado no hub")
	}

	lock.Lock()
	defer lock.Unlock()
	return conn.WriteMessage(messageType, data)
}

// BroadcastMapChunk envia um chunk completo para todos os clientes
func (h *Hub) BroadcastMapChunk(chunkX, chunkY, chunkZ int32, voxelData []byte) {
	msg := &fvnet.MapChunkMessage{
		ChunkX:    chunkX,
		ChunkY:    chunkY,
		ChunkZ:    chunkZ,
		VoxelData: voxelData,
	}
	payload, _ := proto.Marshal(msg)
	envelope := &fvnet.Envelope{
		Type:    fvnet.Envelope_MAP_CHUNK,
		Payload: payload,
	}
	data, _ := proto.Marshal(envelope)
	h.broadcast <- data
}

// BroadcastVegetation envia apenas os deltas de vegetação de um chunk
func (h *Hub) BroadcastVegetation(chunkX, chunkY, chunkZ int32, plants []dfproto.PlantDetail) {
	msg := &fvnet.VegetationUpdateMessage{
		ChunkX: chunkX,
		ChunkY: chunkY,
		ChunkZ: chunkZ,
	}
	for _, p := range plants {
		msg.Plants = append(msg.Plants, fvnet.PlantDetail{
			X:        p.Pos.X,
			Y:        p.Pos.Y,
			MatType:  p.Material.MatType,
			MatIndex: p.Material.MatIndex,
		})
	}

	payload := msg.Marshal() // Usando marshal customizado de vegetation.go
	envelope := &fvnet.Envelope{
		Type:    fvnet.Envelope_VEGETATION_UPDATE,
		Payload: payload,
	}
	data, _ := proto.Marshal(envelope)
	h.broadcast <- data
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Configurar Log em Arquivo para depuração de crash
	if err := os.MkdirAll("servidor/tmp", 0755); err == nil {
		logFile, err := os.OpenFile("servidor/tmp/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			// MultiWriter para logar no console e no arquivo simultaneamente
			mw := io.MultiWriter(os.Stdout, logFile)
			log.SetOutput(mw)
		}
	}
	log.Println("╔══════════════════════════════════════╗")
	log.Println("║    FortressVision SERVER v0.1.0      ║")
	log.Println("╚══════════════════════════════════════╝")

	hub := newHub()
	go hub.run()

	// Inicializar Store (SQLite)
	store := mapdata.NewMapDataStore()

	// Conectar ao DFHack
	dfHost := "localhost:5000"
	if h := os.Getenv("DFHACK_HOST"); h != "" {
		dfHost = h
	}

	log.Printf("Conectando ao DFHack em %s...", dfHost)
	dfClient, err := dfhack.NewClient(dfHost)
	if err != nil {
		log.Fatalf("Erro fatal: não foi possível conectar ao DFHack: %v", err)
	}
	defer dfClient.Close()

	if err := dfClient.FetchStaticData(); err != nil {
		log.Printf("Aviso: Falha ao carregar dados estáticos iniciais: %v", err)
	} else {
		// Inicializar persistência no SQLite com o nome do mundo
		worldName := dfClient.MapInfo.WorldNameEn
		if worldName == "" {
			worldName = dfClient.MapInfo.WorldName
		}
		if worldName != "" {
			log.Printf("Inicializando banco de dados para o mundo: %s", worldName)
			if err := store.OpenInitialize(worldName); err != nil {
				log.Printf("Erro ao abrir SQLite: %v", err)
			}
		}
	}

	// Iniciar Scanner
	scanner := NewServerScanner(dfClient, store, hub)
	scanner.Start()

	// Iniciar Broadcast de Status do Mundo
	go broadcastWorldStatus(hub, dfClient)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, dfClient, store, w, r)
	})

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("Servidor FortressVision iniciado na porta %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}

func handleWebSocket(hub *Hub, dfClient *dfhack.Client, store *mapdata.MapDataStore, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Erro no upgrade do WebSocket: %v", err)
		return
	}
	hub.register <- conn

	// Enviar status inicial
	status := &fvnet.ServerStatus{
		Message:     "Conectado ao Servidor FortressVision",
		DfConnected: dfClient.IsConnected(),
	}
	hub.SendProtoMessage(conn, fvnet.Envelope_SERVER_STATUS, status)

	// Enviar Dicionários de Tipos (Essencial para o Cliente renderizar)
	if dfClient.IsConnected() {
		if dfClient.TiletypeList != nil {
			hub.SendProtoMessage(conn, fvnet.Envelope_TILETYPE_LIST, dfClient.TiletypeList)
		}
		if dfClient.MaterialList != nil {
			hub.SendProtoMessage(conn, fvnet.Envelope_MATERIAL_LIST, dfClient.MaterialList)
		}
	}

	go func() {
		defer func() {
			hub.unregister <- conn
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Erro ao ler mensagem: %v", err)
				break
			}

			// Decodificar Envelope
			var envelope fvnet.Envelope
			if err := proto.Unmarshal(message, &envelope); err != nil {
				log.Printf("Erro ao desempacotar envelope: %v", err)
				continue
			}

			handleClientMessage(hub, conn, dfClient, store, &envelope)
		}
	}()
}

func handleClientMessage(hub *Hub, conn *websocket.Conn, dfClient *dfhack.Client, store *mapdata.MapDataStore, env *fvnet.Envelope) {
	switch env.Type {
	case fvnet.Envelope_PING:
		hub.SendProtoMessage(conn, fvnet.Envelope_PONG, nil)
	case fvnet.Envelope_CLIENT_REQUEST_REGION:
		var req fvnet.ClientRequestRegion
		if err := proto.Unmarshal(env.Payload, &req); err != nil {
			log.Printf("Erro ao ler RequestRegion: %v", err)
			return
		}
		log.Printf("[Network] Cliente solicitou região: Center(%d, %d, %d) Radius:%d", req.CenterX, req.CenterY, req.CenterZ, req.Radius)

		// Buscar chunks no store e enviar
		log.Printf("[WS] Iniciando streaming de região para cliente...")
		go streamRegionToClient(hub, conn, dfClient, store, req)
	}
}

func streamRegionToClient(hub *Hub, conn *websocket.Conn, dfClient *dfhack.Client, store *mapdata.MapDataStore, req fvnet.ClientRequestRegion) {
	// Definir limites
	minX := req.CenterX - req.Radius
	maxX := req.CenterX + req.Radius
	minY := req.CenterY - req.Radius
	maxY := req.CenterY + req.Radius
	z := req.CenterZ

	chunksSent := 0
	// Iterar em blocos de 16
	for x := minX; x <= maxX; x += 16 {
		for y := minY; y <= maxY; y += 16 {
			origin := util.DFCoord{X: x, Y: y, Z: z}.BlockCoord()

			store.Mu.RLock()
			chunk, exists := store.Chunks[origin]
			store.Mu.RUnlock()

			if !exists {
				var err error
				chunk, err = store.LoadChunk(origin)
				if err != nil {
					// Fallback On-Demand: Tenta buscar os blocos faltantes diretamente na memória do DFHack
					if dfClient != nil && dfClient.IsConnected() {
						list, rpcErr := dfClient.GetBlockList(origin.X, origin.Y, origin.Z, origin.X+15, origin.Y+15, origin.Z, 300)
						if rpcErr == nil && list != nil {
							for _, block := range list.MapBlocks {
								store.StoreSingleBlock(&block) // Salva no banco e insere no cache (Chunks map)
							}
							// Tenta pegar o chunk novamente após o processamento
							store.Mu.RLock()
							chunk, exists = store.Chunks[origin]
							store.Mu.RUnlock()
						}
					}

					// Se continuou não existindo (ex: fora dos limites do mapa ou vazio), segue em frente
					if !exists {
						continue
					}
				}
			}

			if chunk != nil {
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				if err := enc.Encode(chunk.Tiles); err != nil {
					log.Printf("[WS] Erro ao codificar tiles para chunk (%d,%d,%d): %v", origin.X, origin.Y, origin.Z, err)
					continue
				}

				msg := &fvnet.MapChunkMessage{
					ChunkX:    origin.X,
					ChunkY:    origin.Y,
					ChunkZ:    origin.Z,
					VoxelData: buf.Bytes(),
				}
				hub.SendProtoMessage(conn, fvnet.Envelope_MAP_CHUNK, msg)
				chunksSent++
			}
		}
	}
	log.Printf("[WS] Streaming concluído: %d chunks enviados para Z=%d", chunksSent, z)
}

func broadcastWorldStatus(hub *Hub, dfClient *dfhack.Client) {
	months := []string{"Granito", "Slate", "Felsite", "Hematita", "Malaquita", "Galena", "Calcário", "Arenito", "Madeira", "Moonstone", "Opal", "Obsidiana"}
	seasons := []string{"Primavera", "Verão", "Outono", "Inverno"}

	for {
		if !dfClient.IsConnected() {
			time.Sleep(5 * time.Second)
			continue
		}

		status := &fvnet.WorldStatus{}

		// 1. Tempo e Nome do Mundo
		world, err := dfClient.GetWorldMapCenter()
		if err == nil {
			status.WorldName = world.NameEn
			status.Year = world.CurYear
			tick := world.CurYearTick
			monthIdx := tick / 33600
			status.Day = (tick%33600)/1200 + 1

			if monthIdx >= 0 && monthIdx < 12 {
				status.Month = months[monthIdx]
				status.Season = seasons[monthIdx/3]
			}
		}

		// 2. População
		units, err := dfClient.GetUnitList()
		if err == nil {
			count := 0
			for _, u := range units.CreatureList {
				if u.IsValid {
					count++
				}
			}
			status.Population = int32(count)
		}
		// 3. Sincronização de Visão (Z-Sync Inteligente Unificado)
		status.ViewZ = dfClient.GetInterestZ()
		view, err := dfClient.GetViewInfo()
		if err == nil {
			status.ViewX = view.ViewPosX + view.ViewSizeX/2
			status.ViewY = view.ViewPosY + view.ViewSizeY/2
		}

		// Enviar para todos os clientes
		payload, _ := proto.Marshal(status)
		envelope := &fvnet.Envelope{
			Type:    fvnet.Envelope_WORLD_STATUS,
			Payload: payload,
		}
		data, _ := proto.Marshal(envelope)
		hub.broadcast <- data

		time.Sleep(200 * time.Millisecond)
	}
}

func (h *Hub) SendProtoMessage(conn *websocket.Conn, msgType fvnet.Envelope_Type, msg interface{}) {
	var payload []byte
	var err error
	if msg != nil {
		if m, ok := msg.(interface{ Marshal() ([]byte, error) }); ok {
			payload, err = m.Marshal()
		} else if pm, ok := msg.(proto.Message); ok {
			payload, err = proto.Marshal(pm)
		}
		if err != nil {
			log.Printf("Erro ao serializar payload: %v", err)
			return
		}
	}

	envelope := &fvnet.Envelope{
		Type:    msgType,
		Payload: payload,
	}

	data, err := proto.Marshal(envelope)
	if err != nil {
		log.Printf("Erro ao serializar envelope: %v", err)
		return
	}

	if err := h.WriteSafe(conn, websocket.BinaryMessage, data); err != nil {
		log.Printf("Erro ao enviar mensagem: %v", err)
	}
}
