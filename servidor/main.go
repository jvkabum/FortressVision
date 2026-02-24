package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"FortressVision/servidor/internal/dfhack"
	"FortressVision/shared/mapdata"
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
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
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
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Cliente registrado: %s", client.RemoteAddr())
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Printf("Cliente desregistrado: %s", client.RemoteAddr())
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Printf("Erro ao enviar para cliente: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
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
	sendProtoMessage(conn, fvnet.Envelope_SERVER_STATUS, status)

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

			handleClientMessage(conn, dfClient, store, &envelope)
		}
	}()
}

func handleClientMessage(conn *websocket.Conn, dfClient *dfhack.Client, store *mapdata.MapDataStore, env *fvnet.Envelope) {
	switch env.Type {
	case fvnet.Envelope_PING:
		sendProtoMessage(conn, fvnet.Envelope_PONG, nil)
	case fvnet.Envelope_CLIENT_REQUEST_REGION:
		var req fvnet.ClientRequestRegion
		if err := proto.Unmarshal(env.Payload, &req); err != nil {
			log.Printf("Erro ao ler RequestRegion: %v", err)
			return
		}
		log.Printf("Cliente solicitou região: Center(%d, %d, %d) Radius:%d", req.CenterX, req.CenterY, req.CenterZ, req.Radius)

		// Buscar chunks no store e enviar
		go streamRegionToClient(conn, store, req)
	}
}

func streamRegionToClient(conn *websocket.Conn, store *mapdata.MapDataStore, req fvnet.ClientRequestRegion) {
	// Definir limites
	minX := req.CenterX - req.Radius
	maxX := req.CenterX + req.Radius
	minY := req.CenterY - req.Radius
	maxY := req.CenterY + req.Radius
	z := req.CenterZ

	// Iterar em blocos de 16
	for x := minX; x <= maxX; x += 16 {
		for y := minY; y <= maxY; y += 16 {
			origin := util.DFCoord{X: x, Y: y, Z: z}.BlockCoord()

			store.Mu.RLock()
			chunk, exists := store.Chunks[origin]
			store.Mu.RUnlock()

			if !exists {
				// Tenta carregar do SQLite se o servidor não tiver na RAM
				var err error
				chunk, err = store.LoadChunk(origin)
				if err != nil {
					continue
				}
			}

			if chunk != nil {
				// Serializar chunk.Tiles para bytes usando GOB (mesmo formato do SQLite)
				// Isso permite que o cliente use o mesmo leitor que o MapDataStore usava
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				if err := enc.Encode(chunk.Tiles); err != nil {
					log.Printf("Erro ao codificar tiles para rede: %v", err)
					continue
				}

				msg := &fvnet.MapChunkMessage{
					ChunkX:    origin.X,
					ChunkY:    origin.Y,
					ChunkZ:    origin.Z,
					VoxelData: buf.Bytes(),
				}
				sendProtoMessage(conn, fvnet.Envelope_MAP_CHUNK, msg)
			}
		}
	}
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

		// 3. Sincronização de Visão (Z-Sync Inteligente)
		view, err := dfClient.GetViewInfo()
		if err == nil {
			zToUse := view.ViewPosZ
			centerX := view.ViewPosX + view.ViewSizeX/2
			centerY := view.ViewPosY + view.ViewSizeY/2

			// Se estiver no céu ou nível muito alto, busca solo via unidades
			if zToUse > 100 {
				units, err := dfClient.GetUnitList()
				if err == nil && len(units.CreatureList) > 0 {
					var bestUnitZ int32 = -1
					minDist := 999999.0
					for _, u := range units.CreatureList {
						if u.PosZ < 150 { // Foca em anões reais abaixo do céu
							dx := float64(u.PosX - centerX)
							dy := float64(u.PosY - centerY)
							dist := dx*dx + dy*dy
							if dist < minDist {
								minDist = dist
								bestUnitZ = u.PosZ
							}
						}
					}
					if bestUnitZ != -1 {
						zToUse = bestUnitZ
					}
				}
			}

			status.ViewX = centerX
			status.ViewY = centerY
			status.ViewZ = zToUse
		}

		// Enviar para todos os clientes
		payload, _ := proto.Marshal(status)
		envelope := &fvnet.Envelope{
			Type:    fvnet.Envelope_WORLD_STATUS,
			Payload: payload,
		}
		data, _ := proto.Marshal(envelope)
		hub.broadcast <- data

		time.Sleep(10 * time.Second)
	}
}

func sendProtoMessage(conn *websocket.Conn, msgType fvnet.Envelope_Type, msg proto.Message) {
	var payload []byte
	var err error
	if msg != nil {
		payload, err = proto.Marshal(msg)
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

	err = conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		log.Printf("Erro ao enviar mensagem: %v", err)
	}
}
