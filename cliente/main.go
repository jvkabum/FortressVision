package main

import (
	"fmt"
	"math"

	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/core"
	"FortressVision/cliente/internal/hud"
	"FortressVision/cliente/internal/network"
	"FortressVision/cliente/internal/world"
	"FortressVision/shared/config"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"

	"kaijuengine.com/engine"
	"kaijuengine.com/engine/cameras"
	"kaijuengine.com/engine/collision"
	"kaijuengine.com/engine/systems/console"
	"kaijuengine.com/matrix"
	"kaijuengine.com/registry/shader_data_registry"
	"kaijuengine.com/rendering"
)

func createTestTerrain(host *engine.Host) {
	const size = 32
	verts := make([]rendering.Vertex, 0)
	indices := make([]uint32, 0)

	for z := 0; z < size; z++ {
		for x := 0; x < size; x++ {
			y := float32(0)
			if (x+z)%2 == 0 {
				y = 0.2
			}
			verts = append(verts, rendering.Vertex{
				Position: matrix.NewVec3(matrix.Float(x), matrix.Float(y), matrix.Float(z)),
				Normal:   matrix.Vec3Up(),
				Color:    matrix.Color{0.2, 0.8, 0.2, 1.0},
			})
		}
	}

	for z := 0; z < size-1; z++ {
		for x := 0; x < size-1; x++ {
			i0 := uint32(z*size + x)
			i1 := uint32(z*size + (x + 1))
			i2 := uint32((z+1)*size + x)
			i3 := uint32((z+1)*size + (x + 1))
			indices = append(indices, i0, i2, i1, i1, i2, i3)
		}
	}

	mesh := host.MeshCache().Mesh("test_terrain", verts, indices)
	mat, _ := host.MaterialCache().Material("basic.material")
	sd := shader_data_registry.Create("basic")

	trans := &matrix.Transform{}
	trans.SetupRawTransform()

	draw := rendering.Drawing{
		Mesh:       mesh,
		Material:   mat,
		ShaderData: sd,
		Transform:  trans,
		ViewCuller: &renderVC{Camera: host.PrimaryCamera()},
	}
	host.Drawings.AddDrawing(draw)
}


type renderVC struct {
	Camera cameras.Camera
}

func (v *renderVC) IsInView(box collision.AABB) bool { return true }
func (v *renderVC) ViewChanged() bool                { return false }

type ConstructionApp struct {
	core.App
	cam   *camera.Controller
	hud   *hud.HUD
	net   *network.Client
	world *world.Manager
	cfg   *config.Config

	lastRequestPos util.DFCoord
}

func (a *ConstructionApp) Launch(host *engine.Host) {
	a.App.Launch(host)
	console.For(host)

	// 1. Carregar Configurações
	a.cfg = config.Load()

	// 2. Inicializar Módulos de Dados
	a.world = world.NewManager()
	a.cam = camera.New(host)
	a.hud = hud.New(host)

	// 3. Configurar Rede (A "Estrada")
	a.net = network.NewClient(a.cfg.ServerURL)
	a.net.OnEnvelope = a.world.HandleEnvelope

	// 4. Conectar HUD aos eventos do Mundo
	a.world.OnWorldStatus = func(s *fvnet.WorldStatus) {
		a.hud.UpdateWorld(s.WorldName)
		a.hud.UpdatePop(int(s.Population))
		a.hud.UpdateSeason(fmt.Sprintf("Ano %d", s.Year))
	}
	a.world.OnStatusMsg = func(msg string, dfConnected bool) {
		fmt.Printf("[App] Status do Servidor: %s (DF: %v)\n", msg, dfConnected)
	}

	// 5. Iniciar Conexão (Assíncrona para não travar o boot)
	go a.net.Connect()

	createTestTerrain(host)

	host.Updater.AddUpdate(func(dt float64) {
		dt32 := float32(dt)
		a.cam.HandleInput(dt32, &host.Window.Keyboard, &host.Window.Mouse)
		a.cam.Update(dt32)

		a.hud.Update(dt, a.cam.TargetLookAt)

		// 6. Solicitar região baseada no foco da câmera
		camPos := util.DFCoord{
			X: int32(a.cam.TargetLookAt[0]),
			Y: int32(a.cam.TargetLookAt[1]),
			Z: int32(a.cam.TargetLookAt[2]),
		}

		// Solicita se mudou de bloco ou a cada ~1 segundo
		if camPos.X != a.lastRequestPos.X || camPos.Z != a.lastRequestPos.Z || math.Abs(dt) > 1.0 {
			a.world.RequestRegion(a.net.Send, camPos, a.cfg.DrawRangeSide)
			a.lastRequestPos = camPos
		}
	})
}

func main() {
	core.Start(&ConstructionApp{})
}
