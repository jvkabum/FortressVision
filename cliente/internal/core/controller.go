package core

import (
	"fmt"

	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/hud"
	"FortressVision/cliente/internal/mesher"
	"FortressVision/cliente/internal/network"
	"FortressVision/cliente/internal/render"
	"FortressVision/cliente/internal/world"
	"FortressVision/shared/config"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"

	"kaijuengine.com/engine"
	"kaijuengine.com/platform/hid"
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
	// O servidor pode ainda estar carregando o mundo quando o cliente abre.
	// Se a conexao cair durante a navegacao, o cliente tenta voltar.
	n.OnDisconnect = func() {
		go n.Connect()
	}
	m := mesher.NewManager()
	r := render.NewTerrainRenderer(host, m)
	r.SetStore(w.Store)
	s := world.NewSyncManager(w, n, c, cfg)

	// Configurar a ponte de eventos (Bridge) entre os subsistemas
	SetupBridge(BridgeConfig{
		Net:      n,
		World:    w,
		Mesher:   m,
		HUD:      h,
		Camera:   c,
		Renderer: r,
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
	ac.handleItemSelection()

	// 2. Gestão de Interface (HUD e Painéis)
	ac.HUD.Update(dt, ac.Camera.TargetLookAt)

	// 3. Renderização de Terreno (Submissão de Desenhos para GPU)
	ac.Renderer.Render()

	// 4. Sincronização de Região (Monitoramento de Posição DF)
	ac.Sync.Update()
}

// handleItemSelection seleciona o item sob o cursor quando o botão esquerdo
// é pressionado. A seleção acontece depois da atualização da câmera para que
// o raio use a posição/rotação exibida no frame atual.
func (ac *AppController) handleItemSelection() {
	mouse := &ac.Host.Window.Mouse
	if !mouse.Pressed(hid.MouseButtonLeft) {
		return
	}

	ray := ac.Host.PrimaryCamera().RayCast(mouse.ScreenPosition())
	selectionRay := util.Ray{
		Origin: util.Vector3{
			X: float32(ray.Origin[0]),
			Y: float32(ray.Origin[1]),
			Z: float32(ray.Origin[2]),
		},
		Direction: util.Vector3{
			X: float32(ray.Direction[0]),
			Y: float32(ray.Direction[1]),
			Z: float32(ray.Direction[2]),
		},
	}

	item, found := ac.World.Store.FindItemByRay(selectionRay, 2000)
	if found {
		ac.HUD.UpdateInspectItem(ac.World.Store.ItemDisplayName(item), item.StackSize)
		itemPos := util.DFCoord{X: item.Pos.X, Y: item.Pos.Y, Z: item.Pos.Z}
		if tile := ac.World.Store.GetTile(itemPos); tile != nil {
			ac.HUD.UpdateInspect(
				fmt.Sprintf("TT %d | M %d:%d", tile.TileType, tile.Material.MatType, tile.Material.MatIndex),
				fmt.Sprintf("Item / %s (%d)", tileShapeName(tile.Shape()), tile.Shape()),
				int(tile.WaterLevel),
				int(tile.MagmaLevel),
			)
		} else {
			ac.HUD.UpdateInspect("--", "Item / sem tile", 0, 0)
		}
		return
	}

	tile, coord, foundTile := ac.World.Store.FindTileByRay(selectionRay, 2000)
	if foundTile {
		ac.HUD.UpdateInspectItem(ac.World.Store.TileDisplayName(tile), 0)
		ac.HUD.UpdateInspect(
			fmt.Sprintf("TT %d | M %d:%d", tile.TileType, tile.Material.MatType, tile.Material.MatIndex),
			fmt.Sprintf("%s (%d) @ %d,%d,%d", tileShapeName(tile.Shape()), tile.Shape(), coord.X, coord.Y, coord.Z),
			int(tile.WaterLevel),
			int(tile.MagmaLevel),
		)
		return
	}
	ac.HUD.UpdateInspectItem("", 0)
	ac.HUD.UpdateInspect("--", "--", 0, 0)
}

func tileShapeName(shape dfproto.TiletypeShape) string {
	switch shape {
	case dfproto.ShapeNoShape:
		return "Desconhecido"
	case dfproto.ShapeEmpty:
		return "Ar"
	case dfproto.ShapeFloor:
		return "Piso"
	case dfproto.ShapeWall:
		return "Parede"
	case dfproto.ShapeFortification:
		return "Fortificacao"
	case dfproto.ShapeRamp:
		return "Rampa"
	case dfproto.ShapeRampTop:
		return "Topo da rampa"
	case dfproto.ShapeStairUp:
		return "Escada cima"
	case dfproto.ShapeStairDown:
		return "Escada baixo"
	case dfproto.ShapeStairUpDown:
		return "Escada dupla"
	case dfproto.ShapeBoulder:
		return "Pedregulho"
	case dfproto.ShapePebbles:
		return "Pedregulhos"
	case dfproto.ShapeEndlessPit:
		return "Poco infinito"
	case dfproto.ShapeTreeShape:
		return "Copa da arvore"
	case dfproto.ShapeTrunkBranch:
		return "Tronco"
	case dfproto.ShapeSapling:
		return "Muda"
	case dfproto.ShapeShrub:
		return "Arbusto"
	case dfproto.ShapeBranch:
		return "Galho"
	case dfproto.ShapeTwig:
		return "Graveto"
	default:
		return "Outro"
	}
}
