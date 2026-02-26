package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
		broadcast:  make(chan []byte, 4096), // Bufferizado para evitar deadlocks e bloqueios
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Hub] Recuperado de pânico fatal: %v", r)
			// Reinicia o loop se necessário ou loga o erro crítico
		}
	}()

	for {
		select {
		case client, ok := <-h.register:
			if !ok {
				return
			}
			h.mu.Lock()
			h.clients[client] = &sync.Mutex{}
			h.mu.Unlock()
			log.Printf("Cliente registrado: %s", client.RemoteAddr())
		case client, ok := <-h.unregister:
			if !ok {
				return
			}
			h.mu.Lock()
			if lock, ok := h.clients[client]; ok {
				lock.Lock()
				delete(h.clients, client)
				client.Close()
				lock.Unlock()
				log.Printf("Cliente desregistrado: %s", client.RemoteAddr())
			}
			h.mu.Unlock()
		case message, ok := <-h.broadcast:
			if !ok {
				return
			}
			h.mu.Lock()
			// Criamos uma lista de clientes para iterar fora do lock do hub
			type clientEntry struct {
				conn *websocket.Conn
				lock *sync.Mutex
			}
			var targets []clientEntry
			for c, l := range h.clients {
				targets = append(targets, clientEntry{c, l})
			}
			h.mu.Unlock()

			for _, target := range targets {
				target.lock.Lock()
				err := target.conn.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Printf("Erro ao enviar para cliente %s: %v", target.conn.RemoteAddr(), err)
					target.conn.Close()
					h.mu.Lock()
					delete(h.clients, target.conn)
					h.mu.Unlock()
				}
				target.lock.Unlock()
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
		// Log silendiado para não poluir se o cliente acabou de desconectar
		return fmt.Errorf("cliente não encontrado no hub")
	}

	lock.Lock()
	defer lock.Unlock()
	return conn.WriteMessage(messageType, data)
}

// safeSend envia para o canal de broadcast protegendo contra pânicos de canal fechado
func (h *Hub) safeSend(data []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Hub] Aviso: Falha ao enviar broadcast (canal fechado?): %v", r)
		}
	}()
	// IMPORTANTE: Não segurar h.mu.Lock() aqui, pois o h.broadcast <- data pode bloquear
	// se o buffer estiver cheio, e o run() precisaria do lock para esvaziar o buffer.
	h.broadcast <- data
}

// BroadcastMapChunk envia um chunk completo para todos os clientes
func (h *Hub) BroadcastMapChunk(chunkX, chunkY, chunkZ int32, voxelData []byte) {
	msg := &fvnet.MapChunkMessage{
		ChunkX:    chunkX,
		ChunkY:    chunkY,
		ChunkZ:    chunkZ,
		VoxelData: voxelData,
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("[Hub] Erro ao serializar chunk: %v", err)
		return
	}
	envelope := &fvnet.Envelope{
		Type:    fvnet.Envelope_MAP_CHUNK,
		Payload: payload,
	}
	data, _ := proto.Marshal(envelope)
	h.safeSend(data)
}

// BroadcastVegetation envia apenas os deltas de vegetação de um chunk
func (h *Hub) BroadcastVegetation(chunkX, chunkY, chunkZ int32, plants []dfproto.PlantDetail) {
	if h == nil {
		return
	}
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

	payload := msg.Marshal() // Corrigido para msg.Marshal()
	envelope := &fvnet.Envelope{
		Type:    fvnet.Envelope_VEGETATION_UPDATE,
		Payload: payload,
	}
	data, _ := proto.Marshal(envelope)
	h.safeSend(data)
}

// BroadcastServerStatus envia uma mensagem de status/notificação para todos os clientes
func (h *Hub) BroadcastServerStatus(message string, dfConnected bool) {
	msg := &fvnet.ServerStatus{
		Message:     message,
		DfConnected: dfConnected,
	}
	payload, _ := proto.Marshal(msg)
	envelope := &fvnet.Envelope{
		Type:    fvnet.Envelope_SERVER_STATUS,
		Payload: payload,
	}
	data, _ := proto.Marshal(envelope)
	h.broadcast <- data
}

func main() {
	// Garante que o working directory é o mesmo diretório do executável,
	// para que caminhos relativos (saves/, tmp/) funcionem corretamente.
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		os.Chdir(exeDir)
	}

	log.SetFlags(log.Ltime | log.Lshortfile)

	// Configurar Log em Arquivo para depuração de crash
	if err := os.MkdirAll("tmp", 0755); err == nil {
		logFile, err := os.OpenFile("tmp/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	dfHost := "127.0.0.1:5000"
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
			log.Printf("[Startup] Dimensões do Mapa: %dx%dx%d blocos (DF-Blocks)",
				dfClient.MapInfo.BlockSizeX, dfClient.MapInfo.BlockSizeY, dfClient.MapInfo.BlockSizeZ)
			log.Printf("Inicializando banco de dados para o mundo: %s", worldName)
			if err := store.OpenInitialize(worldName); err != nil {
				log.Printf("Erro ao abrir SQLite: %v", err)
			}

			// Carregar Construções Iniciais (Fase 6) - Assíncrono para retorno rápido
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Startup-Buildings] Recuperado de pânico: %v", r)
					}
				}()
				log.Println("[Startup] Sincronizando construções do mundo...")
				bList, err := dfClient.GetBuildingList()
				if err == nil && bList != nil {
					for _, b := range bList.BuildingList {
						instance := &mapdata.BuildingInstance{
							Index:     b.Index,
							MinPos:    util.DFCoord{X: b.PosXMin, Y: b.PosYMin, Z: b.PosZMin},
							MaxPos:    util.DFCoord{X: b.PosXMax, Y: b.PosYMax, Z: b.PosZMax},
							Direction: b.Direction,
						}
						store.AddBuilding(instance)
					}
					log.Printf("  → %d construções indexadas", len(bList.BuildingList))
				}
			}()
		}
	}

	// Iniciar Scanner
	scanner := NewServerScanner(dfClient, store, hub)
	scanner.Start()

	// ---------------------------------------------------------
	// Sincronização Dinâmica de Unidades (Fase 6)
	// ---------------------------------------------------------
	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Units-Loop] Recuperado de pânico: %v", r)
					}
				}()
				if dfClient.IsConnected() {
					units, err := dfClient.GetUnitList()
					if err == nil && units != nil {
						for _, u := range units.CreatureList {
							// Converter para nossa estrutura interna
							instance := &mapdata.UnitInstance{
								ID:     u.ID,
								Name:   u.Name,
								Race:   u.Race,
								Pos:    util.DFCoord{X: u.PosX, Y: u.PosY, Z: u.PosZ},
								SubPos: util.Vector3{X: u.SubposX, Y: u.SubposY, Z: u.SubposZ},
								Flags1: u.Flags1,
								Flags2: u.Flags2,
								Flags3: u.Flags3,
								IsDead: !u.IsValid,
							}
							store.UpdateUnit(instance)
						}
					}
				}
			}()
			time.Sleep(1 * time.Second) // Unidades pedem atualização mais frequente
		}
	}()

	// ---------------------------------------------------------
	// Auto-Save Periodico e Limpeza de Memória (Purge)
	// ---------------------------------------------------------
	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[AutoSave-Loop] Recuperado de pânico: %v", r)
					}
				}()
				if dfClient.IsConnected() && dfClient.MapInfo != nil {
					worldName := dfClient.MapInfo.WorldNameEn
					if worldName == "" {
						worldName = dfClient.MapInfo.WorldName
					}

					// Salva chunks sujos
					store.Save(worldName) //nolint:errcheck — background save, log de erro já está no persistence

					// Purga chunks distantes do foco atual (Raio de 256 tiles)
					viewZ := dfClient.GetInterestZ()
					view, err := dfClient.GetViewInfo()
					if err == nil && view != nil {
						centerX := view.ViewPosX + view.ViewSizeX/2
						centerY := view.ViewPosY + view.ViewSizeY/2
						center := util.DFCoord{X: centerX, Y: centerY, Z: viewZ}
						store.Purge(center, 512.0) // Raio de 512 tiles (~32 blocos)
					}
				}
			}()
			time.Sleep(30 * time.Second)
		}
	}()

	// ---------------------------------------------------------
	// Heurística de Mundo Novo (Smart Full-Scan)
	// ---------------------------------------------------------
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Startup-Heuristic] Recuperado de pânico: %v", r)
			}
		}()
		// Damos um tempo menor (2 segs) para o servidor conectar e carregar MapInfo
		time.Sleep(2 * time.Second)
		if dfClient.MapInfo == nil {
			return
		}

		count, err := store.GetChunkCount()
		if err == nil {
			log.Printf("[Startup] Inspeção de Banco de Dados: %d chunks persistidos.", count)
			if count < 5000 {
				log.Println("[Startup] Banco incompleto. Agendando Varredura Total...")
				// Notifica o cliente IMEDIATAMENTE para evitar timeout da splash screen
				hub.BroadcastServerStatus("FULL_SCAN:0/1", dfClient.IsConnected())
				scanner.StartFullScan()
			} else {
				log.Println("[Startup] Banco ok. Scan direcional ativo.")
			}
		}
	}()

	// Iniciar Broadcast de Status do Mundo
	go broadcastWorldStatus(hub, dfClient)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r, dfClient, store, scanner)
	})

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Iniciar Servidor HTTP/WebSocket com verificação de porta
	addr := "127.0.0.1:" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("╔══════════════════════════════════════════════════════════════╗")
		log.Printf("║ ERRO CRÍTICO: Não foi possível abrir a porta %s.      ║", port)
		log.Printf("║ Provavelmente há outra instância do servidor rodando.        ║")
		log.Printf("║ Tente fechar o FortressVision.exe e o server.exe             ║")
		log.Printf("╚══════════════════════════════════════════════════════════════╝")
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	ln.Close() // Fecha para o ListenAndServe reabrir

	log.Printf("Servidor FortressVision iniciado em %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Erro fatal no servidor HTTP: %v", err)
	}
}

// serveWs maneja requisições websocket do peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request, dfClient *dfhack.Client, store *mapdata.MapDataStore, scanner *ServerScanner) {
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

			handleClientMessage(hub, conn, dfClient, store, &envelope, scanner)
		}
	}()
}

func handleClientMessage(hub *Hub, conn *websocket.Conn, dfClient *dfhack.Client, store *mapdata.MapDataStore, env *fvnet.Envelope, scanner *ServerScanner) {
	switch env.Type {
	case fvnet.Envelope_PING:
		hub.SendProtoMessage(conn, fvnet.Envelope_PONG, nil)
	case fvnet.Envelope_CLIENT_REQUEST_REGION:
		var req fvnet.ClientRequestRegion
		if err := proto.Unmarshal(env.Payload, &req); err != nil {
			log.Printf("Erro ao ler RequestRegion: %v", err)
			return
		}
		log.Printf("[Network] Região Center(%d,%d,%d) R:%d", req.CenterX, req.CenterY, req.CenterZ, req.Radius)
		dfClient.SetInterestZ(req.CenterZ)
		go streamRegionToClient(hub, conn, dfClient, store, &req, scanner)
	}
}

func streamRegionToClient(hub *Hub, conn *websocket.Conn, dfClient *dfhack.Client, store *mapdata.MapDataStore, req *fvnet.ClientRequestRegion, scanner *ServerScanner) {
	// Streaming agora é permitido mesmo durante o Full Scan para uma experiência fluida (Fase 8)

	// Definir limites
	minX := req.CenterX - req.Radius
	maxX := req.CenterX + req.Radius
	minY := req.CenterY - req.Radius
	maxY := req.CenterY + req.Radius
	z := req.CenterZ

	chunksSent := 0
	chunksEmpty := 0
	// Ajustar limites para alinhar com a grade de 16-tiles
	startX := (minX / 16) * 16
	startY := (minY / 16) * 16

	// Iterar em blocos de 16
	for x := startX; x <= maxX; x += 16 {
		for y := startY; y <= maxY; y += 16 {
			origin := util.DFCoord{X: x, Y: y, Z: z}.BlockCoord()

			chunk, exists := store.GetChunk(origin)

			if !exists {
				var err error
				chunk, err = store.LoadChunk(origin)
				if err != nil {
					// Fallback On-Demand: Tenta buscar os blocos faltantes diretamente na memória do DFHack
					if dfClient != nil && dfClient.IsConnected() && dfClient.MapInfo != nil {
						info := dfClient.MapInfo
						// Verifica se a coordenada está dentro dos limites reais da fortaleza no mundo (Absoluto)
						if origin.X < info.BlockPosX*16 || origin.X >= (info.BlockPosX+info.BlockSizeX)*16 ||
							origin.Y < info.BlockPosY*16 || origin.Y >= (info.BlockPosY+info.BlockSizeY)*16 ||
							origin.Z < info.BlockPosZ || origin.Z >= info.BlockPosZ+info.BlockSizeZ {
							// Fora dos limites do mapa gerado, é vazio.
							store.MarkAsEmpty(origin)
							continue
						}

						log.Printf("[WS-Fallback] Chunk %v não encontrado. Requisitando ao DFHack...", origin)
						// Converter Tile Coord (Origin) de volta para Block Index para a RPC
						bx, by := origin.X/16, origin.Y/16
						list, rpcErr := dfClient.GetBlockList(bx, by, origin.Z, bx, by, origin.Z, 1)
						if rpcErr == nil && list != nil && len(list.MapBlocks) > 0 {
							for _, block := range list.MapBlocks {
								store.StoreSingleBlock(&block)
							}
							chunk, exists = store.GetChunk(origin)
							if exists {
								log.Printf("[WS-Fallback] Chunk %v recuperado com sucesso via DFHack! (Enviando ao cliente)", origin)
							}
						} else {
							// Se rpcErr for nil mas list vazio, logar também
							if rpcErr == nil {
								log.Printf("[WS-Fallback] Chunk %v retornou VAZIO do DFHack (Céu/Ar). Memorizando...", origin)
								store.MarkAsEmpty(origin)
							} else {
								log.Printf("[WS-Fallback] ERRO ao recuperar %v: %v", origin, rpcErr)
							}
						}
					}
					if !exists {
						chunksEmpty++
						// Notifica o cliente que o chunk é "Ar" (vazio) para progresso de loading
						msg := &fvnet.MapChunkMessage{
							ChunkX:    origin.X,
							ChunkY:    origin.Y,
							ChunkZ:    origin.Z,
							VoxelData: nil, // VoxelData nil = Chunk Vazio/Ar
						}
						hub.SendProtoMessage(conn, fvnet.Envelope_MAP_CHUNK, msg)
						continue
					}
				} else {
					// Blocos carregados do SQLite devem voltar pro Cache ativo para evitar hit de disco repetido.
					store.Mu.Lock()
					if store.Chunks == nil {
						store.Chunks = make(map[util.DFCoord]*mapdata.Chunk)
					}
					store.Chunks[origin] = chunk
					store.Mu.Unlock()
				}
			}

			if chunk != nil {
				// Se for um bloco conhecido como vazio (Ar), enviamos sem VoxelData
				if chunk.IsEmpty {
					chunksEmpty++
					msg := &fvnet.MapChunkMessage{
						ChunkX:    origin.X,
						ChunkY:    origin.Y,
						ChunkZ:    origin.Z,
						VoxelData: nil,
					}
					hub.SendProtoMessage(conn, fvnet.Envelope_MAP_CHUNK, msg)
					continue
				}

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
	if chunksSent > 0 {
		log.Printf("[WS] Streaming → %d chunks enviados, %d ar/céu (Z=%d)", chunksSent, chunksEmpty, z)
	}
}

func broadcastWorldStatus(hub *Hub, dfClient *dfhack.Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WorldStatus] Recuperado de pânico: %v", r)
			// Reinicia após uma pausa
			go func() {
				time.Sleep(5 * time.Second)
				broadcastWorldStatus(hub, dfClient)
			}()
		}
	}()

	months := []string{"Granito", "Slate", "Felsite", "Hematita", "Malaquita", "Galena", "Calcário", "Arenito", "Madeira", "Moonstone", "Opal", "Obsidiana"}
	seasons := []string{"Primavera", "Verão", "Outono", "Inverno"}

	for {
		if !dfClient.IsConnected() || dfClient.MapInfo == nil {
			time.Sleep(5 * time.Second)
			continue
		}

		status := &fvnet.WorldStatus{}

		// 1. Tempo e Nome do Mundo
		world, err := dfClient.GetWorldMapCenter()
		if err == nil && world != nil {
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
		if err == nil && units != nil {
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
		if dfClient.MapInfo != nil {
			status.ZOffset = dfClient.MapInfo.BlockPosZ
		}
		view, err := dfClient.GetViewInfo()
		if err == nil && view != nil {
			status.ViewX = view.ViewPosX + view.ViewSizeX/2
			status.ViewY = view.ViewPosY + view.ViewSizeY/2
		}

		// Enviar para todos os clientes usando o método seguro
		payload, _ := proto.Marshal(status)
		envelope := &fvnet.Envelope{
			Type:    fvnet.Envelope_WORLD_STATUS,
			Payload: payload,
		}
		data, _ := proto.Marshal(envelope)
		hub.safeSend(data)

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
