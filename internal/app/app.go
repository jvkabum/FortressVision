package app

import (
	"fmt"
	"log"
	"math"
	"runtime"

	"FortressVision/internal/camera"
	"FortressVision/internal/config"
	"FortressVision/internal/dfhack"
	"FortressVision/internal/mapdata"
	"FortressVision/internal/meshing"
	"FortressVision/internal/render"
	"FortressVision/internal/util"
	"FortressVision/pkg/dfproto"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// AppState representa os estados possíveis da aplicação.
type AppState int

const (
	StateLoading    AppState = iota // Carregando assets
	StateMenu                       // Menu principal
	StateConnecting                 // Conectando ao DFHack
	StateViewing                    // Visualizando o mapa
	StatePaused                     // Pausado
)

// App é a aplicação principal do FortressVision.
type App struct {
	Config *config.Config
	State  AppState

	// Controlador de Câmera (Novo Sistema)
	Cam *camera.CameraController

	// Informações de debug
	frameCount int
	tileInfo   string

	// Dados do mapa e comunicação
	mapCenter     util.DFCoord
	dfClient      *dfhack.Client
	mapStore      *mapdata.MapDataStore
	matStore      *mapdata.MaterialStore
	mesher        *meshing.BlockMesher
	resultStore   *meshing.ResultStore
	scanner       *MapScanner
	renderer      *render.Renderer
	isUpdatingMap bool // Flag para evitar sobreposição de sync de mapa

	lastManualMove   int64   // Timestamp da última interação manual do usuário
	lastZKeyTime     float64 // Timestamp da última mundança de nível Z (repetição suave)
	initialZSyncDone bool

	lastAutoSaveTime float64 // Timestamp do último auto-save

	// Estado da Splash Screen
	Loading                bool
	LoadingStatus          string
	LoadingProgress        float32
	LoadingTotalBlocks     int  // Total de blocos esperados na carga inicial
	LoadingProcessedBlocks int  // Total já processados e enviados para GPU
	FullScanActive         bool // Flag para download total do mundo
}

// New cria uma nova instância da aplicação.
func New(cfg *config.Config) *App {
	app := &App{
		Config:          cfg,
		State:           StateLoading,
		mapCenter:       util.NewDFCoord(0, 0, 10), // Força início no nível 10
		Loading:         true,
		LoadingStatus:   "Conectando ao DFHack...",
		LoadingProgress: 0.1,
	}
	return app
}

// Run inicia o loop principal da aplicação.
func (a *App) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro fatal recuperado: %v", r)
			// Tenta dar um dump do stack trace ou apenas espera para o usuário ver
			panic(r) // Re-throw para o Windows mostrar o erro se necessário
		}
	}()

	// Inicializar janela raylib
	rl.SetConfigFlags(rl.FlagMsaa4xHint | rl.FlagWindowResizable)
	rl.InitWindow(a.Config.WindowWidth, a.Config.WindowHeight, a.Config.WindowTitle)

	if a.Config.Fullscreen {
		rl.ToggleFullscreen()
	}

	rl.SetTargetFPS(a.Config.TargetFPS)

	// Inicializar sistema de câmera
	a.Cam = camera.New()
	// Configura posição inicial
	a.Cam.SetTarget(rl.Vector3{X: 0, Y: 0, Z: float32(a.mapCenter.Z) * util.GameScale})

	log.Println("[FortressVision] Janela inicializada com sucesso")
	log.Printf("[FortressVision] Resolução: %dx%d", a.Config.WindowWidth, a.Config.WindowHeight)

	a.State = StateViewing

	// Inicializar sistemas novos
	a.mapStore = mapdata.NewMapDataStore()
	a.matStore = mapdata.NewMaterialStore()
	a.resultStore = meshing.NewResultStore()

	workers := runtime.NumCPU() // Poder máximo de processamento
	if workers < 4 {
		workers = 4
	}

	log.Printf("[App] Iniciando Mesher com %d workers (CPU Cores: %d)", workers, runtime.NumCPU())
	a.mesher = meshing.NewBlockMesher(workers, a.matStore, a.resultStore)
	a.renderer = render.NewRenderer()
	a.scanner = NewMapScanner(a)

	// Iniciar threads de background
	go a.connectDFHack()

	// Loop principal
	for !rl.WindowShouldClose() {
		a.update()
		a.draw()
	}

	// Cleanup
	a.shutdown()
	rl.CloseWindow()
}

// update atualiza a lógica do jogo a cada frame.
func (a *App) update() {
	a.frameCount++

	switch a.State {
	case StateViewing:
		a.renderer.ProcessPurge() // Limpeza incremental da GPU
		// RAM Streaming: Executa a purga apenas a cada 120 frames (2s a 60fps)
		if a.frameCount%120 == 0 {
			// Purga desativada para Unlimited Vision
			// a.mapStore.Purge(a.mapCenter, 256.0)
		}
		a.handleAutoSave() // Salvamento periódico (SQLite)
		a.updateDFSync()
		a.updateCamera()
		a.updateInput()
		a.updateMap()
		a.processMesherResults()
	}
}

func (a *App) updateDFSync() {
	if a.dfClient == nil || !a.dfClient.IsConnected() {
		return
	}

	// Se o usuário moveu a câmera manualmente nos últimos 5 segundos, NÃO sincroniza
	if rl.GetTime()-float64(a.lastManualMove)/1000.0 < 5.0 {
		return
	}

	// Sincroniza a cada 60 frames (1 segundo)
	if a.frameCount%60 == 0 {
		view, err := a.dfClient.GetViewInfo()
		if err == nil {
			// 1. Sincronização Inicial de Z (apenas uma vez para "pousar" no solo)
			if !a.initialZSyncDone {
				log.Printf("[DFHack] Sincronização Inicial: ViewZ=%d FollowUnit=%d", view.ViewPosZ, view.FollowUnitID)

				zToUse := view.ViewPosZ

				// Busca por unidades reais (ignora o céu Z=163)
				units, err := a.dfClient.GetUnitList()
				foundGround := false
				if err == nil && len(units.CreatureList) > 0 {
					var bestUnitZ int32 = -1
					minDist := float64(999999)
					centerX := view.ViewPosX + view.ViewSizeX/2
					centerY := view.ViewPosY + view.ViewSizeY/2

					for _, u := range units.CreatureList {
						// Ignora unidades no céu
						if u.PosZ > 150 {
							continue
						}
						dx := float64(u.PosX - centerX)
						dy := float64(u.PosY - centerY)
						dist := dx*dx + dy*dy
						if dist < minDist {
							minDist = dist
							bestUnitZ = u.PosZ
							foundGround = true
						}
					}
					if foundGround {
						zToUse = bestUnitZ
						log.Printf("[DFHack] Solo detectado via unidade próxima em Z=%d", zToUse)
					}
				}

				// Se não achou unidade, tenta o cursor se estiver abaixo do céu
				if !foundGround && view.CursorPosZ > 0 && view.CursorPosZ < 150 {
					zToUse = view.CursorPosZ
					log.Printf("[DFHack] Solo detectado via cursor em Z=%d", zToUse)
					foundGround = true
				}

				a.mapCenter.Z = zToUse
				a.initialZSyncDone = true
			}

			// 2. Sincronização de Z (Estratégia Armok Vision)
			// Priorizamos units (anões) se a visualização do DF estiver muito alta (Sky/Mountain top)
			zToUse := view.ViewPosZ

			// Se estivermos em um nível muito alto ou vazio, tentamos achar o "solo vivo"
			if zToUse > 100 {
				units, err := a.dfClient.GetUnitList()
				if err == nil && len(units.CreatureList) > 0 {
					var bestUnitZ int32 = -1
					minDist := float64(999999)
					centerX := view.ViewPosX + view.ViewSizeX/2
					centerY := view.ViewPosY + view.ViewSizeY/2

					for _, u := range units.CreatureList {
						// Ignora unidades no céu, foca em anões reais
						if u.PosZ < 150 {
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
						// log.Printf("[DFHack] Focando no solo via unidade: Z=%d (DF View era %d)", zToUse, view.ViewPosZ)
					}
				}
			}

			if a.mapCenter.Z != zToUse {
				log.Printf("[DFHack] Ajuste de Z: %d -> %d", a.mapCenter.Z, zToUse)
				a.mapCenter.Z = zToUse
			}

			// Atualiza posição alvo
			targetCoord := util.NewDFCoord(
				view.ViewPosX+view.ViewSizeX/2,
				view.ViewPosY+view.ViewSizeY/2,
				a.mapCenter.Z,
			)

			newTarget := util.DFToWorldPos(targetCoord)

			// Só sincroniza se estiver muito longe (evita briga com movimento manual)
			dist := rl.Vector3Distance(a.Cam.TargetLookAt, newTarget)
			if dist > 50.0 || !a.initialZSyncDone {
				a.Cam.SetTarget(newTarget)
				a.initialZSyncDone = true
			}
		}
	}
}

// connectDFHack tenta conectar ao DFHack em background.
func (a *App) connectDFHack() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro em connectDFHack: %v", r)
		}
	}()

	client, err := dfhack.NewClient(fmt.Sprintf("%s:%d", a.Config.DFHackHost, a.Config.DFHackPort))
	if err != nil {
		log.Printf("[DFHack] Erro ao conectar: %v", err)
		a.LoadingStatus = "Erro: DFHack não encontrado. Pressione ESPAÇO para modo Offline."
		a.LoadingProgress = 0.5
		return
	}

	log.Println("[DFHack] Conectado com sucesso!")
	a.dfClient = client

	// Carregar dados estáticos
	if err := a.dfClient.FetchStaticData(); err != nil {
		log.Printf("[DFHack] Erro ao buscar dados estáticos: %v", err)
		return
	}

	// Atualizar tiletypes e materiais no store
	a.mapStore.UpdateTiletypes(a.dfClient.TiletypeList)
	a.matStore.UpdateMaterials(a.dfClient.MaterialList)

	// Carregar save existente se houver
	worldName := a.dfClient.MapInfo.WorldNameEn
	if worldName == "" {
		worldName = a.dfClient.MapInfo.WorldName
	}
	if worldName != "" {
		log.Printf("[App] Verificando persistência para o mundo: %s", worldName)
		if err := a.mapStore.Load(worldName); err != nil {
			log.Printf("[App] Erro ao carregar save: %v", err)
		} else {
			// Marcar todos os chunks carregados para o renderer re-enviar se necessário
			// Na verdade, o Mesher vai detectar que MTime > 0 e que o Renderer não tem o modelo.
		}
	}

	// Verificar se o mundo já tem dados salvos
	if a.mapStore.HasData() {
		log.Println("[App] Mundo já possui dados salvos. Iniciando modo streaming normal.")
		a.scanner.Start()
	} else {
		log.Println("[App] Mundo NOVO detectado. Iniciando download total (Deep Scan)...")
		a.Loading = true // Garante que a tela de loading apareça para o FullScan
		a.scanner.StartFullScan()
	}

	// Sincronização inicial de foco (Focal Point)
	view, err := a.dfClient.GetViewInfo()
	if err == nil {
		targetX := view.ViewPosX + view.ViewSizeX/2
		targetY := view.ViewPosY + view.ViewSizeY/2
		targetZ := view.ViewPosZ

		// ESTRATÉGIA DE BUSCA DE UNIDADES (Z=37 vs Z=163)
		foundReference := false

		// 1. Tentar FollowUnitID padrão
		if view.FollowUnitID != -1 {
			units, err := a.dfClient.GetUnitList()
			if err == nil {
				for _, u := range units.CreatureList {
					if u.ID == view.FollowUnitID {
						log.Printf("[DFHack] Foco via FollowUnitID: Z=%d", u.PosZ)
						targetX, targetY, targetZ = u.PosX, u.PosY, u.PosZ
						foundReference = true
						break
					}
				}
			}
		}

		// 2. Se falhou (comum em Fortress Mode se não houver unidade seguida), buscar anões manualmente
		if !foundReference {
			units, err := a.dfClient.GetUnitList()
			if err == nil && len(units.CreatureList) > 0 {
				log.Printf("[DFHack] Buscando anões/unidades em %d criaturas...", len(units.CreatureList))

				var bestUnit *dfproto.UnitDefinition
				minDist := float64(999999)

				// No Adventure Mode, a câmera do DF (ViewPos) costuma estar centralizada NO jogador,
				// mesmo que o FollowUnitID esteja bugado.
				centerX := view.ViewPosX + view.ViewSizeX/2
				centerY := view.ViewPosY + view.ViewSizeY/2
				log.Printf("[DFHack] Centro da Visão: (%d, %d). Buscando unidade mais próxima...", centerX, centerY)

				for i := range units.CreatureList {
					u := &units.CreatureList[i]
					log.Printf("  -> Unidade ID=%d em Z=%d (X=%d Y=%d) Válida:%v", u.ID, u.PosZ, u.PosX, u.PosY, u.IsValid)

					// Ignora unidades em níveis de céu
					if u.PosZ > 200 {
						continue
					}

					// Calcula distância 2D até o centro da visão
					dx := float64(u.PosX - centerX)
					dy := float64(u.PosY - centerY)
					dist := dx*dx + dy*dy

					if dist < minDist {
						minDist = dist
						bestUnit = u
					}
				}

				if bestUnit != nil {
					log.Printf("[DFHack] Unidade de referência detectada: ID=%d em Z=%d (Distância: %.1f)", bestUnit.ID, bestUnit.PosZ, math.Sqrt(minDist))
					targetX, targetY, targetZ = bestUnit.PosX, bestUnit.PosY, bestUnit.PosZ
					foundReference = true
				}
			}
		}

		// 3. Fallback: Cursor (se não encontrou unidade)
		if !foundReference && view.CursorPosZ > 0 && view.CursorPosZ < 1000 {
			targetZ = view.CursorPosZ
			log.Printf("[DFHack] Usando cursor para foco inicial: Z=%d", targetZ)
		}

		log.Printf("[DFHack] Resultado da sincronização inicial: X=%d Y=%d Z=%d (Unidade Detectada:%v)", targetX, targetY, targetZ, foundReference)
		a.mapCenter.X = targetX
		a.mapCenter.Y = targetY
		a.mapCenter.Z = targetZ
		a.initialZSyncDone = true

		// Move a câmera para o alvo imediatamente
		if a.Cam != nil {
			targetPos := util.DFToWorldPos(a.mapCenter)
			a.Cam.SetTarget(targetPos)
		}
	}
}

// handleAutoSave verifica se é hora de salvar o progresso automaticamente.
func (a *App) handleAutoSave() {
	currentTime := rl.GetTime()
	// Salva a cada 60 segundos
	if currentTime-a.lastAutoSaveTime >= 60.0 {
		a.lastAutoSaveTime = currentTime

		if a.dfClient != nil && a.dfClient.IsConnected() {
			worldName := a.dfClient.MapInfo.WorldNameEn
			if worldName == "" {
				worldName = a.dfClient.MapInfo.WorldName
			}
			if worldName != "" {
				log.Printf("[Auto-Save] Iniciando salvamento automático para o mundo: %s", worldName)
				// Rodar em goroutine para não travar o jogo
				go a.mapStore.Save(worldName)
			}
		}
	}
}

func (a *App) updateMap() {
	if a.dfClient == nil || !a.dfClient.IsConnected() {
		return
	}

	// throttle: requisita a cada 60 frames (1s a 60fps) para manter o FPS estável
	if a.frameCount%60 != 0 {
		return
	}

	// Evita múltiplas buscas simultâneas
	if a.isUpdatingMap {
		return
	}
	if a.dfClient == nil || !a.dfClient.IsConnected() {
		return
	}

	a.isUpdatingMap = true

	// Determina a posição central atual baseada na câmera
	center := util.WorldToDFCoord(a.Cam.CurrentLookAt)
	center.Z = a.mapCenter.Z

	// Atualiza o centro visual para o Mesher (Oclusão)
	a.mapCenter.X = center.X
	a.mapCenter.Y = center.Y

	// Purge de modelos distantes (Hysteresis)
	a.renderer.Purge(center, 120.0)

	// Rodar busca em background para não travar a thread de renderização
	go func(centerPos util.DFCoord) {
		defer func() { a.isUpdatingMap = false }()

		// DFHack RemoteFortressReader exige coordenadas em BLOCOS (1 bloco = 16x16 tiles)
		// O Scanner (Background) lida com o mapa todo. O "updateMap" atualiza só uma bolha muito estreita
		// (água caindo, anão construindo pareder na visão Imediata) para a CPU focar em animar.
		radiusInBlocks := int32(2) // Reduzido drasticamente para 2 (32x32 tiles de atualização dinâmica)
		minX := (centerPos.X / 16) - radiusInBlocks
		maxX := (centerPos.X / 16) + radiusInBlocks
		minY := (centerPos.Y / 16) - radiusInBlocks
		maxY := (centerPos.Y / 16) + radiusInBlocks

		// Limitando Z para uma fatia fininha "perto da câmera" em vez de puxar 30 andares por frame.
		minZ := centerPos.Z - 3
		maxZ := centerPos.Z + 1

		if a.dfClient != nil && a.dfClient.MapInfo != nil {
			if maxZ >= a.dfClient.MapInfo.BlockSizeZ {
				maxZ = a.dfClient.MapInfo.BlockSizeZ - 1
			}
		}

		if minX < 0 {
			minX = 0
		}
		if minY < 0 {
			minY = 0
		}

		list, err := a.dfClient.GetBlockList(
			minX, minY, minZ,
			maxX, maxY, maxZ,
			1000,
		)
		if err != nil {
			log.Printf("[App] Erro ao buscar blocos: %v", err)
			return
		}

		if len(list.MapBlocks) > 0 {
			log.Printf("[DFHack] Recebidos %d blocos", len(list.MapBlocks))
			a.mapStore.StoreBlocks(list)

			enqueuedCount := 0
			for i := range list.MapBlocks {
				block := &list.MapBlocks[i]
				origin := util.DFCoord{X: block.MapX, Y: block.MapY, Z: block.MapZ}

				a.mapStore.Mu.RLock()
				chunk, exists := a.mapStore.Chunks[origin]
				a.mapStore.Mu.RUnlock()

				if !exists || a.mesher == nil {
					continue
				}

				// VERIFICAÇÃO RÍGIDA DE PERFORMANCE (STUTTER FIX 3)
				// Só envia para a thread de geometria se o chunk "sujou" (MTime subiu) no SQLite/Memória E a GPU está desatualizada.
				// Como GetModelVersion já devolve a versão que a GPU tem, se ela for >= MTime do chunk, pula!
				gpuVersion := a.renderer.GetModelVersion(origin)
				if gpuVersion < chunk.MTime {
					if a.mesher.Enqueue(meshing.Request{
						Origin:   origin,
						Data:     a.mapStore,
						FocusZ:   int(a.mapCenter.Z),
						MaxDepth: 130, // Aumentado para alcançar o bedrock de qualquer lugar
						MTime:    chunk.MTime,
					}) {
						enqueuedCount++
					}
				}
			}

			if enqueuedCount > 0 {
				log.Printf("[Mesher] %d novos blocos enfileirados", enqueuedCount)
				if a.Loading && a.LoadingTotalBlocks == 0 {
					a.LoadingTotalBlocks = enqueuedCount
					a.LoadingProcessedBlocks = 0
				}
			}
		} else if a.Loading && a.LoadingTotalBlocks == 0 {
			log.Println("[App] Aguardando blocos da fortaleza...")
		}
	}(center)
}

// processMesherResults consome resultados da fila e envia para a GPU.
func (a *App) processMesherResults() {
	// Durante o loading, podemos gastar bastante tempo por frame subindo malha
	// Durante o jogo (StateViewing), aplicamos um "Time Slicing" (Fatiamento de Tempo) rígido para evitar Stutters.
	// 1 frame a 60FPS = 16.6ms. Vamos dedicar no máximo 4ms para upload de malha por frame.
	timeBudget := 0.004 // 4 milissegundos
	if a.Loading {
		timeBudget = 0.500 // 500ms durante a tela de loading para agilizar
	}

	startTime := rl.GetTime()

	for {
		// Se já estouramos o orçamento de tempo deste frame, paramos e deixamos pro próximo.
		if rl.GetTime()-startTime > timeBudget {
			break
		}

		select {
		case res := <-a.mesher.Results():
			if len(res.Terreno.Vertices) > 0 || len(res.Liquidos.Vertices) > 0 {
				log.Printf("[Renderer] Upload de Geometria: %s (Terreno: %d, Água: %d vértices)",
					res.Origin.String(), len(res.Terreno.Vertices)/3, len(res.Liquidos.Vertices)/3)
			}
			a.renderer.UploadResult(res)

			// Lógica de progresso do Loading
			if a.Loading {
				a.LoadingProcessedBlocks++

				// Calcula progresso visual
				if a.LoadingTotalBlocks > 0 {
					a.LoadingProgress = float32(a.LoadingProcessedBlocks) / float32(a.LoadingTotalBlocks)
					a.LoadingStatus = fmt.Sprintf("Construindo terreno: %d/%d (%.1f%%)",
						a.LoadingProcessedBlocks, a.LoadingTotalBlocks, a.LoadingProgress*100)
				}

				// Só encerra o loading quando processarmos quase tudo (95%)
				// Ou se o usuário apertar SPACE (tratado no updateInput)
				if a.LoadingTotalBlocks > 0 && float32(a.LoadingProcessedBlocks)/float32(a.LoadingTotalBlocks) >= 0.95 {
					a.Loading = false
					a.LoadingProgress = 1.0
					log.Println("Loading concluído automaticamente! Iniciando renderização.")
				}
			}
		default:
			// Não há mais resultados prontos na fila, sai do loop imediatamente
			return
		}
	}
}

// updateCamera atualiza a câmera baseado no input.
func (a *App) updateCamera() {
	dt := rl.GetFrameTime()

	// Processa input (WASD, Mouse, Zoom)
	if a.Cam.HandleInput(dt) {
		a.lastManualMove = int64(rl.GetTime() * 1000) // Converte segundos para ms
	}

	// Atualiza física/interpolação da câmera
	a.Cam.Update(dt)

	// Nível Z com Q/E (REPETIÇÃO CONTÍNUA AO SEGURAR)
	zRepeatDelay := 0.08 // Velocidade de descida/subida rápida
	if rl.IsKeyPressed(rl.KeyE) || (rl.IsKeyDown(rl.KeyE) && rl.GetTime()-a.lastZKeyTime > zRepeatDelay) {
		a.mapCenter.Z++
		a.Cam.TargetLookAt.Y += util.GameScale
		a.lastZKeyTime = rl.GetTime()
	}
	if rl.IsKeyPressed(rl.KeyQ) || (rl.IsKeyDown(rl.KeyQ) && rl.GetTime()-a.lastZKeyTime > zRepeatDelay) {
		a.mapCenter.Z--
		a.Cam.TargetLookAt.Y -= util.GameScale
		a.lastZKeyTime = rl.GetTime()
	}

	// Alternar projeção com P
	if rl.IsKeyPressed(rl.KeyP) {
		if a.Cam.Mode == camera.ModePerspective {
			a.Cam.SetMode(camera.ModeOrthographic)
			log.Println("[Camera] Modo Ortográfico")
		} else {
			a.Cam.SetMode(camera.ModePerspective)
			log.Println("[Camera] Modo Perspectiva")
		}
	}
}

// updateInput processa entradas de teclado gerais.
func (a *App) updateInput() {
	// Toggle debug info
	if rl.IsKeyPressed(rl.KeyF3) {
		a.Config.ShowDebugInfo = !a.Config.ShowDebugInfo
	}

	// Pular Loading manualmente
	if a.Loading && rl.IsKeyPressed(rl.KeySpace) {
		log.Println("[App] Loading/Download pulado manualmente pelo usuário.")
		a.Loading = false
		a.FullScanActive = false
		// Se parou o download total, inicia o streaming normal para não ficar no vácuo
		a.scanner.Start()
	}

	// Toggle grid
	if rl.IsKeyPressed(rl.KeyG) {
		a.Config.ShowGrid = !a.Config.ShowGrid
	}

	// FORÇAR DOWNLOAD TOTAL (Deep Scan)
	if rl.IsKeyPressed(rl.KeyF6) {
		log.Println("[App] Forçando Download Total (Deep Scan) via F6...")
		if !a.FullScanActive {
			a.Loading = true
			a.scanner.StartFullScan()
		}
	}

	// Save Manual
	if rl.IsKeyPressed(rl.KeyF5) {
		if a.dfClient != nil && a.dfClient.IsConnected() {
			worldName := a.dfClient.MapInfo.WorldNameEn
			if worldName == "" {
				worldName = a.dfClient.MapInfo.WorldName
			}
			if worldName != "" {
				log.Println("[App] Iniciando salvamento manual...")
				go a.mapStore.Save(worldName)
			}
		}
	}

	// Toggle wireframe
	if rl.IsKeyPressed(rl.KeyF4) {
		a.Config.WireframeMode = !a.Config.WireframeMode
	}

	// Fullscreen toggle
	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleFullscreen()
	}
}

// draw renderiza a cena.
func (a *App) draw() {
	rl.BeginDrawing()
	rl.ClearBackground(rl.NewColor(30, 30, 40, 255))

	if a.Loading {
		a.drawLoadingScreen()
	} else {
		a.drawScene()
		a.drawHUD()
	}

	rl.EndDrawing()
}

// drawScene renderiza a cena 3D.
func (a *App) drawScene() {
	rl.BeginMode3D(a.Cam.RLCamera)

	// Grid de referência
	if a.Config.ShowGrid {
		rl.DrawGrid(40, util.GameScale)
	}

	// Renderizar modelos do mapa real
	if a.renderer != nil {
		a.renderer.Draw(a.Cam.RLCamera.Position, a.mapCenter.Z)
	}

	rl.EndMode3D()
}

// drawTestTerrain desenha um terreno de teste para validar o sistema.
// Será substituído pelo sistema de meshing real na Fase 4.
func (a *App) drawTestTerrain() {
	groundLevel := float32(a.mapCenter.Z) * util.GameScale

	// Plano de chão
	for x := int32(-10); x < 10; x++ {
		for y := int32(-10); y < 10; y++ {
			pos := util.DFToWorldPos(util.NewDFCoord(x, y, a.mapCenter.Z))

			// Variar a cor baseado na posição (padrão xadrez)
			var color rl.Color
			if (x+y)%2 == 0 {
				color = rl.NewColor(80, 120, 80, 255) // Verde escuro
			} else {
				color = rl.NewColor(100, 140, 100, 255) // Verde claro
			}

			rl.DrawCube(
				rl.Vector3{
					X: pos.X + util.GameScale*0.5,
					Y: groundLevel - util.GameScale*0.5,
					Z: pos.Z - util.GameScale*0.5,
				},
				util.GameScale*0.98, util.GameScale, util.GameScale*0.98,
				color,
			)
		}
	}

	// Algumas paredes de teste
	wallPositions := []util.DFCoord{
		{X: -5, Y: -5, Z: a.mapCenter.Z},
		{X: -4, Y: -5, Z: a.mapCenter.Z},
		{X: -3, Y: -5, Z: a.mapCenter.Z},
		{X: -5, Y: -4, Z: a.mapCenter.Z},
		{X: -5, Y: -3, Z: a.mapCenter.Z},
		// Segundo andar
		{X: -5, Y: -5, Z: a.mapCenter.Z + 1},
		{X: -4, Y: -5, Z: a.mapCenter.Z + 1},
	}

	for _, wc := range wallPositions {
		pos := util.DFToWorldPos(wc)
		rl.DrawCube(
			rl.Vector3{
				X: pos.X + util.GameScale*0.5,
				Y: pos.Y + util.GameScale*0.5,
				Z: pos.Z - util.GameScale*0.5,
			},
			util.GameScale*0.98, util.GameScale, util.GameScale*0.98,
			rl.NewColor(128, 128, 128, 255), // Cinza pedra
		)
		// Wireframe por cima
		rl.DrawCubeWires(
			rl.Vector3{
				X: pos.X + util.GameScale*0.5,
				Y: pos.Y + util.GameScale*0.5,
				Z: pos.Z - util.GameScale*0.5,
			},
			util.GameScale, util.GameScale, util.GameScale,
			rl.NewColor(60, 60, 60, 255),
		)
	}

	// Água de teste
	waterPos := util.DFToWorldPos(util.NewDFCoord(3, 3, a.mapCenter.Z))
	for dx := int32(0); dx < 3; dx++ {
		for dy := int32(0); dy < 3; dy++ {
			wp := util.DFToWorldPos(util.NewDFCoord(3+dx, 3+dy, a.mapCenter.Z))
			rl.DrawCube(
				rl.Vector3{
					X: wp.X + util.GameScale*0.5,
					Y: waterPos.Y + util.GameScale*0.25,
					Z: wp.Z - util.GameScale*0.5,
				},
				util.GameScale, util.GameScale*0.5, util.GameScale,
				rl.NewColor(50, 100, 200, 128), // Azul translúcido
			)
		}
	}
}

// drawHUD desenha a interface sobreposta.
func (a *App) drawHUD() {
	if !a.Config.ShowDebugInfo {
		return
	}

	// Fundo semi-transparente para o debug
	rl.DrawRectangle(5, 5, 320, 180, rl.NewColor(0, 0, 0, 150))

	// FPS
	fps := rl.GetFPS()
	fpsColor := rl.Green
	if fps < 30 {
		fpsColor = rl.Red
	} else if fps < 50 {
		fpsColor = rl.Yellow
	}
	rl.DrawText(fmt.Sprintf("FPS: %d", fps), 10, 10, 20, fpsColor)

	// Informações da câmera
	rl.DrawText(fmt.Sprintf("Câmera: (%.1f, %.1f, %.1f) Zoom: %.1f",
		a.Cam.RLCamera.Position.X, a.Cam.RLCamera.Position.Y, a.Cam.RLCamera.Position.Z, a.Cam.CurrentZoom),
		10, 35, 16, rl.White)

	// Nível Z atual
	dfViewZ := int32(0)
	if a.dfClient != nil && a.dfClient.MapInfo != nil {
		view, _ := a.dfClient.GetViewInfo()
		if view != nil {
			dfViewZ = view.ViewPosZ
		}
	}

	rl.DrawText(fmt.Sprintf("Z Visual: %d | Z DFHack: %d (Sync: %s)", a.mapCenter.Z, dfViewZ, a.dfClient.IsConnectedString()),
		10, 55, 16, rl.White)

	// Coordenada DF do centro
	dfCoord := util.WorldToDFCoord(a.Cam.CurrentLookAt)
	dfCoord.Z = a.mapCenter.Z
	rl.DrawText(fmt.Sprintf("DF Coord: (%d, %d, %d)", dfCoord.X, dfCoord.Y, dfCoord.Z),
		10, 75, 16, rl.White)

	// Controles
	rl.DrawText("Controles:", 10, 105, 16, rl.NewColor(200, 200, 200, 255))
	rl.DrawText("WASD/Setas: Mover | Scroll: Zoom | F11: Fullscreen", 10, 125, 14, rl.NewColor(170, 170, 170, 255))

	// Feedback Visual do Wireframe (F4)
	wireframeStatus := "[OFF]"
	wireframeColor := rl.NewColor(170, 170, 170, 255)
	if a.Config.WireframeMode {
		wireframeStatus = "[ON]"
		wireframeColor = rl.Orange
	}

	rl.DrawText(fmt.Sprintf("Q/E: Nível Z | G: Grid | F3: Debug | F4: Wireframe %s", wireframeStatus), 10, 143, 14, wireframeColor)
	rl.DrawText("F5: Salvar Mundo (Manual) | F6: Forçar Download Total", 10, 161, 14, rl.NewColor(150, 200, 150, 255))

	// Título no topo
	title := "FortressVision v0.1.0 - Alpha"
	titleWidth := rl.MeasureText(title, 18)
	rl.DrawText(title,
		int32(rl.GetScreenWidth())-titleWidth-10, 10,
		18, rl.NewColor(200, 200, 200, 200))
}

// shutdown realiza a limpeza de recursos.
func (a *App) shutdown() {
	log.Println("[App] Finalizando aplicação...")

	// Salvar progresso automaticamente ao fechar
	if a.dfClient != nil && a.dfClient.IsConnected() {
		worldName := a.dfClient.MapInfo.WorldNameEn
		if worldName == "" {
			worldName = a.dfClient.MapInfo.WorldName
		}
		if worldName != "" {
			log.Printf("[App] Salvando progresso do mundo %s antes de fechar...", worldName)
			a.mapStore.Save(worldName)
		}
	}

	a.mapStore.Close() // Fecha SQLite

	if err := a.Config.Save(); err != nil {
		log.Printf("[FortressVision] Erro ao salvar configurações: %v", err)
	}
}

func (a *App) drawLoadingScreen() {
	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// Fundo elegante (Gradiente Escuro)
	rl.DrawRectangleGradientV(0, 0, screenWidth, screenHeight, rl.NewColor(20, 30, 48, 255), rl.NewColor(36, 59, 85, 255))

	// Título centralizado
	title := "FORTRESS VISION"
	titleWidth := rl.MeasureText(title, 40)
	rl.DrawText(title, screenWidth/2-titleWidth/2, screenHeight/2-60, 40, rl.SkyBlue)

	// Status do carregamento
	status := a.LoadingStatus
	statusWidth := rl.MeasureText(status, 20)
	rl.DrawText(status, screenWidth/2-statusWidth/2, screenHeight/2+20, 20, rl.LightGray)

	// Barra de Progresso
	barWidth := int32(400)
	barHeight := int32(10)
	barX := screenWidth/2 - barWidth/2
	barY := screenHeight/2 + 60

	// Fundo da barra
	rl.DrawRectangle(barX, barY, barWidth, barHeight, rl.DarkGray)
	// Parte preenchida (azul vibrante)
	rl.DrawRectangle(barX, barY, int32(float32(barWidth)*a.LoadingProgress), barHeight, rl.SkyBlue)

	// Dica de Skip (Só aparece se o download total estiver ativo)
	if a.FullScanActive {
		skipMsg := "Pressione ESPAÇO para pular e entrar direto (Streaming)"
		skipWidth := rl.MeasureText(skipMsg, 16)
		rl.DrawText(skipMsg, screenWidth/2-skipWidth/2, screenHeight/2+100, 16, rl.Gray)
	}

	// Pequena animação de progresso (caso algo demore)
	if a.LoadingProgress < 0.95 {
		a.LoadingProgress += (0.95 - a.LoadingProgress) * 0.005
	}
}
