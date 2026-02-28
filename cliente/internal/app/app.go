package app

import (
	"log"
	"runtime"

	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/client"
	"FortressVision/cliente/internal/meshing"
	"FortressVision/cliente/internal/render"
	"FortressVision/shared/config"
	"FortressVision/shared/mapdata"
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

	// Bloco selecionado pelo usuário (Inspeção)
	SelectedCoord *util.DFCoord

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
	WorldName        string
	WorldYear        int32
	WorldSeason      string
	WorldDay         int32
	WorldMonth       string
	WorldPopulation  int
	ZOffset          int32 // Diferença entre coordenada interna e Elevation do HUD
	lastWorldUpdate  float64
	LoadingStartTime float64 // Timestamp de quando a sincronização inicial começou
}

// New cria uma nova instância da aplicação.
func New(cfg *config.Config) *App {
	app := &App{
		Config:           cfg,
		State:            StateLoading,
		mapCenter:        util.NewDFCoord(0, 0, 10), // Força início no nível 10
		Loading:          true,
		LoadingStatus:    "Conectando ao DFHack...",
		LoadingProgress:  0.1,
		LoadingStartTime: rl.GetTime(),
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
	a.mesher = meshing.NewBlockMesher(workers, a.matStore, a.renderer.AssetMgr, a.resultStore)

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
		a.updateMap(false)
		a.processMesherResults()
	case StatePaused:
		a.updateInput() // Permite detectar ESC para despausar
	}
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
