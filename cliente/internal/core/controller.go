package core

import (
	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/hud"
	"FortressVision/cliente/internal/mesher"
	"FortressVision/cliente/internal/network"
	"FortressVision/cliente/internal/render"
	"FortressVision/cliente/internal/world"
	"FortressVision/shared/config"

	"kaijuengine.com/engine"
)

// AppController centraliza a gestão dos subsistemas do FortressVision.
type AppController struct {
	Host     *engine.Host
	Config   *config.Config
	World    *world.Manager
	Camera   *camera.Controller
	HUD      *hud.HUD
	Net      *network.Client
	Mesher   *mesher.Manager
	Renderer *render.TerrainRenderer
	Sync     *world.SyncManager
}

// NewAppController inicializa e conecta todos os serviços do cliente.
func NewAppController(host *engine.Host) *AppController {
	cfg := config.Load()
	mesher.LoadRampGeometries(".") // Pre-carrega geometrias complexas de rampa
	w := world.NewManager()
	c := camera.New(host)
	h := hud.New(host)
	n := network.NewClient(cfg.ServerURL)
	m := mesher.NewManager()
	r := render.NewTerrainRenderer(host, m)
	s := world.NewSyncManager(w, n, c, cfg)

	// Configurar a ponte de eventos (Bridge) entre os subsistemas
	SetupBridge(BridgeConfig{
		Net:    n,
		World:  w,
		Mesher: m,
		HUD:    h,
		Camera: c,
	})

	// Iniciar diagnósticos visuais e conexão de rede
	render.SetupDiagnostics(host)
	go n.Connect()

	return &AppController{
		Host:     host,
		Config:   cfg,
		World:    w,
		Camera:   c,
		HUD:      h,
		Net:      n,
		Mesher:   m,
		Renderer: r,
		Sync:     s,
	}
}

// Update executa a lógica de pulsar (tick) de todos os subsistemas orquestrados.
func (ac *AppController) Update(dt float64) {
	dt32 := float32(dt)

	// 1. Gestão de Câmera (Input e Interpolação)
	ac.Camera.HandleInput(dt32, &ac.Host.Window.Keyboard, &ac.Host.Window.Mouse)
	ac.Camera.Update(dt32)

	// 2. Gestão de Interface (HUD e Painéis)
	ac.HUD.Update(dt, ac.Camera.TargetLookAt)

	// 3. Renderização de Terreno (Submissão de Desenhos para GPU)
	ac.Renderer.Render()

	// 4. Sincronização de Região (Monitoramento de Posição DF)
	ac.Sync.Update()
}
