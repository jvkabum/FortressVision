package app

import (
	"fmt"
	"log"
	"runtime"

	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/client"
	"FortressVision/cliente/internal/meshing"
	"FortressVision/cliente/internal/render"
	"FortressVision/shared/config"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"

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
	mapCenter   util.DFCoord
	netClient   *client.NetworkClient
	mapStore    *mapdata.MapDataStore
	matStore    *mapdata.MaterialStore
	mesher      *meshing.BlockMesher
	resultStore *meshing.ResultStore
	renderer    *render.Renderer

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

	// Estado do Mundo (DFHack)
	WorldName       string
	WorldYear       int32
	WorldSeason     string
	WorldDay        int32
	WorldMonth      string
	WorldPopulation int
	lastWorldUpdate float64
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
	rl.SetTraceLogLevel(rl.LogWarning) // Reduz ruído no terminal

	if a.Config.Fullscreen {
		rl.ToggleFullscreen()
	}

	rl.SetTargetFPS(a.Config.TargetFPS)
	rl.SetExitKey(0) // Desativa o fechamento da janela ao apertar ESC (Fase 10)

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
	a.renderer = render.NewRenderer()
	a.mesher = meshing.NewBlockMesher(workers, a.matStore, a.resultStore)

	// Iniciar threads de background
	go a.connectServer()

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
		a.updateCamera()
		a.updateInput()
		a.updateMap()
		a.processMesherResults()
	case StatePaused:
		a.updateInput() // Permite detectar ESC para despausar
	}
}

// O DFSync agora é feito pelo servidor. O cliente apenas recebe atualizações.

// connectServer tenta conectar ao Servidor FortressVision.
func (a *App) connectServer() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro em connectServer: %v", r)
		}
	}()

	a.netClient = client.NewNetworkClient(a.Config.ServerURL, a.mapStore)

	// Callbacks
	a.netClient.OnStatus = func(msg string, dfConnected bool) {
		log.Printf("[Server] Status: %s (DF: %v)", msg, dfConnected)
	}

	a.netClient.OnMapChunk = func(origin util.DFCoord) {
		a.mapStore.Mu.RLock()
		chunk, exists := a.mapStore.Chunks[origin]
		a.mapStore.Mu.RUnlock()

		if exists && a.mesher != nil {
			a.mesher.Enqueue(meshing.Request{
				Origin:   origin,
				Data:     a.mapStore,
				FocusZ:   int(a.mapCenter.Z),
				MaxDepth: 130,
				MTime:    chunk.MTime,
			})
		}
	}

	a.netClient.OnWorldStatus = func(status *fvnet.WorldStatus) {
		a.WorldName = status.WorldName
		a.WorldYear = status.Year
		a.WorldDay = status.Day
		a.WorldMonth = status.Month
		a.WorldSeason = status.Season
		a.WorldPopulation = int(status.Population)

		// Sincronização automática de foco (Z-Sync)
		if !a.initialZSyncDone || rl.GetTime()-float64(a.lastManualMove)/1000.0 > 5.0 {
			newZ := status.ViewZ
			if a.mapCenter.Z != newZ {
				// Sincroniza o Z do servidor com o Z do app apenas se houver mudança relevante
				if a.mapCenter.Z == 0 || util.Abs(a.mapCenter.Z-status.ViewZ) > 0 {
					log.Printf("[App] Sincronizando Z com Servidor: %d -> %d", a.mapCenter.Z, status.ViewZ)
					a.mapCenter.Z = status.ViewZ
				}

				// Atualiza posição alvo da câmera
				targetCoord := util.NewDFCoord(status.ViewX, status.ViewY, status.ViewZ)
				newTarget := util.DFToWorldPos(targetCoord)

				// Se for a primeira vez ou estiver muito longe, move a câmera
				dist := rl.Vector3Distance(a.Cam.TargetLookAt, newTarget)
				if dist > 50.0 || !a.initialZSyncDone {
					a.Cam.SetTarget(newTarget)
					a.initialZSyncDone = true
				}
			}
		}
	}

	a.netClient.OnTiletypes = func(list *dfproto.TiletypeList) {
		a.mapStore.Mu.Lock()
		for _, tt := range list.TiletypeList {
			item := tt
			a.mapStore.Tiletypes[tt.ID] = &item
		}
		a.mapStore.Mu.Unlock()
		log.Printf("[App] Dicionário de %d tiletypes sincronizado.", len(list.TiletypeList))
	}

	a.netClient.OnMaterials = func(list *dfproto.MaterialList) {
		go func() {
			a.matStore.UpdateMaterials(list)
			log.Printf("[App] Dicionário de %d materiais sincronizado em background.", len(list.MaterialList))
		}()
	}

	if err := a.netClient.Connect(); err != nil {
		log.Printf("[Server] Erro ao conectar: %v", err)
		a.LoadingStatus = "Erro ao conectar ao Servidor. Verifique se o servidor está rodando."
		return
	}

	log.Println("[Network] Conectado ao Servidor FortressVision!")
	a.LoadingStatus = "Sincronizando com o mundo..."
}

// handleAutoSave verifica se é hora de salvar o progresso automaticamente.
func (a *App) handleAutoSave() {
	currentTime := rl.GetTime()
	// Salva a cada 60 segundos
	if currentTime-a.lastAutoSaveTime >= 60.0 {
		a.lastAutoSaveTime = currentTime

		// O Auto-save do SQLite agora é responsabilidade do Servidor.
		// O cliente pode salvar seu próprio cache se for necessário futuramente.
	}
}

func (a *App) updateMap() {
	if a.netClient == nil || !a.netClient.IsConnected() {
		return
	}

	// throttle: requisita a cada 180 frames (3s) normal, ou 90 frames no loading
	checkInterval := 180
	if a.Loading {
		checkInterval = 90
	}
	if a.frameCount%checkInterval != 0 {
		return
	}

	// Determina a posição central atual baseada na câmera
	center := util.WorldToDFCoord(a.Cam.CurrentLookAt)
	center.Z = a.mapCenter.Z

	// Solicita região ao servidor
	radius := int32(64) // Raio de cobertura

	// Inicializa o total esperado para a tela de carregamento (apenas na primeira vez)
	if a.Loading && a.LoadingTotalBlocks == 0 {
		blocksPerSide := (radius * 2) / 16
		a.LoadingTotalBlocks = int(blocksPerSide * blocksPerSide)
		log.Printf("[App] Esperando %d blocos para concluir sincronização inicial", a.LoadingTotalBlocks)
	}

	a.netClient.RequestRegion(center, radius)
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
			if len(res.Terreno.Vertices) > 0 || len(res.Liquidos.Vertices) > 0 || len(res.MaterialGeometries) > 0 {
				log.Printf("[Renderer] Upload de Geometria: %s (Terreno: %d, Água: %d, Texturas: %d tipos)",
					res.Origin.String(), len(res.Terreno.Vertices)/3, len(res.Liquidos.Vertices)/3, len(res.MaterialGeometries))
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

				// Só encerra o loading quando processarmos o suficiente (40% ou pelo menos 10 blocos e 3s)
				loadThreshold := float32(0.40)
				timeSinceSync := rl.GetTime() - startTime // simplistic proxy

				if a.LoadingTotalBlocks > 0 && (float32(a.LoadingProcessedBlocks)/float32(a.LoadingTotalBlocks) >= loadThreshold || (a.LoadingProcessedBlocks >= 10 && timeSinceSync > 3.0)) {
					a.Loading = false
					a.LoadingProgress = 1.0
					log.Printf("[App] Loading concluído! (%d blocos processados). Iniciando renderização.", a.LoadingProcessedBlocks)
				}
			}
		default:
			// Não há mais resultados prontos na fila, sai do loop imediatamente
			return
		}
	}
}

// updateWorldStatus agora pode ser alimentado pelo servidor futuramente.
func (a *App) updateWorldStatus() {}

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
		// No modo C/S, o loading termina quando os primeiros chunks chegam
	}

	// Toggle grid
	if rl.IsKeyPressed(rl.KeyG) {
		a.Config.ShowGrid = !a.Config.ShowGrid
	}

	// Toggle wireframe
	if rl.IsKeyPressed(rl.KeyF4) {
		a.Config.WireframeMode = !a.Config.WireframeMode
	}

	// Toggle weather with F7
	if rl.IsKeyPressed(rl.KeyF7) {
		if a.renderer != nil && a.renderer.Weather != nil {
			newType := (int(a.renderer.Weather.Type) + 1) % 3
			a.renderer.Weather.Type = render.WeatherType(newType)
			log.Printf("[App] Clima alterado para: %v", a.renderer.Weather.Type)
		}
	}

	// Fullscreen toggle
	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleFullscreen()
	}

	// ESC: Alternar Pausa/Menu (Fase 10)
	if rl.IsKeyPressed(rl.KeyEscape) {
		if a.State == StateViewing {
			a.State = StatePaused
			log.Println("[App] Jogo Pausado")
		} else if a.State == StatePaused {
			a.State = StateViewing
			log.Println("[App] Retomando Jogo")
		}
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

		if a.State == StatePaused {
			a.drawPauseMenu()
		}
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

	// Fundo semi-transparente para o debug (Aumentado para Fase 9)
	rl.DrawRectangle(5, 5, 340, 240, rl.NewColor(0, 0, 0, 180))
	rl.DrawRectangleLines(5, 5, 340, 240, rl.NewColor(50, 50, 50, 255))

	// FPS
	fps := rl.GetFPS()
	fpsColor := rl.Green
	if fps < 30 {
		fpsColor = rl.Red
	} else if fps < 50 {
		fpsColor = rl.Yellow
	}
	rl.DrawText(fmt.Sprintf("FPS: %d", fps), 15, 15, 20, fpsColor)

	// Estado do Clima (Novo na Fase 8)
	weatherStr := "Dia Limpo"
	weatherColor := rl.SkyBlue
	if a.renderer != nil && a.renderer.Weather != nil {
		switch a.renderer.Weather.Type {
		case render.WeatherRain:
			weatherStr = "Chuva"
			weatherColor = rl.Blue
		case render.WeatherSnow:
			weatherStr = "Neve"
			weatherColor = rl.White
		}
	}
	rl.DrawText(weatherStr, 220, 15, 20, weatherColor)

	// Divisor
	rl.DrawLine(15, 40, 325, 40, rl.NewColor(100, 100, 100, 100))

	// Informações de Localização
	rl.DrawText("LOCALIZAÇÃO", 15, 50, 12, rl.Gray)

	offsetZ := int32(0)
	// O offset Z agora virá do servidor via metadados do mapa futuramente.

	dfCoord := util.WorldToDFCoord(a.Cam.CurrentLookAt)
	dfCoord.Z = a.mapCenter.Z

	displayX := dfCoord.X
	displayY := dfCoord.Y
	displayZ := dfCoord.Z - offsetZ

	rl.DrawText(fmt.Sprintf("Coord DF: (%d, %d, %d)", displayX, displayY, displayZ), 15, 65, 16, rl.White)

	dfViewZ := int32(0)
	syncStatus := "Offline"
	if a.netClient != nil && a.netClient.IsConnected() {
		syncStatus = "Conectado (Servidor)"
	}
	displayViewZ := dfViewZ - offsetZ

	rl.DrawText(fmt.Sprintf("Elevação: %d (DF: %d) [%s]", displayZ, displayViewZ, syncStatus), 15, 85, 14, rl.LightGray)

	// Divisor
	rl.DrawLine(15, 105, 325, 105, rl.NewColor(100, 100, 100, 100))

	// Info do Mundo (Novo na Fase 9)
	worldStr := fmt.Sprintf("%d, %s %d - %s", a.WorldYear, a.WorldMonth, a.WorldDay, a.WorldSeason)
	if a.WorldName != "" {
		rl.DrawText(a.WorldName, 15, 115, 14, rl.Gold)
	}
	rl.DrawText(worldStr, 15, 130, 14, rl.LightGray)
	rl.DrawText(fmt.Sprintf("População: %d criaturas", a.WorldPopulation), 15, 145, 14, rl.LightGray)

	// Divisor
	rl.DrawLine(15, 165, 325, 165, rl.NewColor(100, 100, 100, 100))

	// Atalhos Rápidos
	rl.DrawText("CONTROLES", 15, 175, 12, rl.Gray)
	rl.DrawText("Q/E: Nível Z | Scroll: Zoom | WASD: Mover", 15, 190, 14, rl.LightGray)

	wireframeExtra := ""
	if a.Config.WireframeMode {
		wireframeExtra = " [WIREFRAME ON]"
	}
	rl.DrawText(fmt.Sprintf("F7: Clima | F11: Tela Cheia | F3: HUD%s", wireframeExtra), 15, 210, 14, rl.SkyBlue)

	// Título no canto inferior direito
	title := "FortressVision v0.1.0 - Alpha"
	titleWidth := rl.MeasureText(title, 18)
	rl.DrawText(title,
		int32(rl.GetScreenWidth())-titleWidth-20, int32(rl.GetScreenHeight())-30,
		18, rl.NewColor(200, 200, 200, 150))
}

// drawPauseMenu desenha o menu de escape centralizado.
func (a *App) drawPauseMenu() {
	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// 1. Fundo escurecido (Dimmer)
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(0, 0, 0, 150))

	// 2. Painel Central
	panelWidth := int32(400)
	panelHeight := int32(300)
	panelX := (screenWidth - panelWidth) / 2
	panelY := (screenHeight - panelHeight) / 2

	rl.DrawRectangle(panelX, panelY, panelWidth, panelHeight, rl.NewColor(30, 30, 35, 255))
	rl.DrawRectangleLines(panelX, panelY, panelWidth, panelHeight, rl.White)

	// Título do Menu
	menuTitle := "MENU DE PAUSA"
	titleWidth := rl.MeasureText(menuTitle, 24)
	rl.DrawText(menuTitle, panelX+(panelWidth-titleWidth)/2, panelY+30, 24, rl.Gold)

	// 3. Botões
	buttonX := panelX + 50
	buttonWidth := panelWidth - 100
	buttonHeight := int32(40)

	// Botão: RETOMAR
	if a.drawButton(buttonX, panelY+90, buttonWidth, buttonHeight, "RETOMAR (ESC)", rl.Green) {
		a.State = StateViewing
	}

	// Botão: CONFIGURAÇÕES (Placeholder/Info)
	if a.drawButton(buttonX, panelY+145, buttonWidth, buttonHeight, "OPÇÕES (F3/F4/F7)", rl.Gray) {
		// Por enquanto exibe apenas info, mas poderia abrir submenu
	}

	// Botão: SAIR
	if a.drawButton(buttonX, panelY+200, buttonWidth, buttonHeight, "SAIR DO JOGO", rl.Red) {
		// Para fechar via código no Raylib/Go, precisamos sinalizar o loop principal
		// mas aqui podemos apenas chamar o cleanup e sair
		a.shutdown()
		log.Println("[App] Encerrando aplicação pelo menu.")
		runtime.Goexit() // Uma forma de "parar" forçado se necessário, mas WindowShouldClose é melhor.
		// Vamos usar uma flag ou apenas forçar o fechamento da janela
		rl.CloseWindow()
	}
}

// drawButton desenha um botão genérico com hover e retorna true se clicado.
func (a *App) drawButton(x, y, w, h int32, text string, color rl.Color) bool {
	mousePos := rl.GetMousePosition()
	isHover := mousePos.X >= float32(x) && mousePos.X <= float32(x+w) &&
		mousePos.Y >= float32(y) && mousePos.Y <= float32(y+h)

	drawColor := color
	if isHover {
		drawColor.R += 30
		drawColor.G += 30
		drawColor.B += 30
		rl.SetMouseCursor(rl.MouseCursorPointingHand)
	} else {
		rl.SetMouseCursor(rl.MouseCursorDefault)
	}

	rl.DrawRectangle(x, y, w, h, rl.NewColor(50, 50, 50, 255))
	rl.DrawRectangleLines(x, y, w, h, drawColor)

	textWidth := rl.MeasureText(text, 18)
	rl.DrawText(text, x+(w-textWidth)/2, y+(h-18)/2, 18, rl.White)

	return isHover && rl.IsMouseButtonPressed(rl.MouseLeftButton)
}

// shutdown realiza a limpeza de recursos.
func (a *App) shutdown() {
	log.Println("[App] Finalizando aplicação...")

	// Salvar progresso automaticamente ao fechar
	// A persistência agora é responsabilidade do servidor.

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
