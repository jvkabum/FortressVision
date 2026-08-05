package core

import (
	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/hud"
	"FortressVision/cliente/internal/mesher"
	"FortressVision/cliente/internal/network"
	"FortressVision/cliente/internal/render"
	"FortressVision/cliente/internal/world"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"
	"fmt"
)

// BridgeConfig contém as instâncias necessárias para configurar a orquestração do app.
type BridgeConfig struct {
	Net      *network.Client
	World    *world.Manager
	Mesher   *mesher.Manager
	HUD      *hud.HUD
	Camera   *camera.Controller
	Renderer *render.TerrainRenderer
}

// SetupBridge estabelece as conexões de eventos entre os diferentes módulos do cliente.
func SetupBridge(cfg BridgeConfig) {
	// 1. Conexão Rede -> Mundo
	cfg.Net.OnEnvelope = func(env *fvnet.Envelope) {
		cfg.World.HandleEnvelope(env)
	}

	// 2. Conexão Mundo -> Mesher (Geometria do Terreno)
	cfg.World.OnMapChunkUpdated = func(chunk *mapdata.Chunk) {
		if chunk != nil {
			if cfg.Renderer != nil {
				cfg.Renderer.UpdateChunkEntities(chunk)
			}
			cfg.Mesher.RequestMeshUpdate(chunk)
		}
	}
	if cfg.Renderer != nil {
		cfg.World.OnUnitsUpdated = cfg.Renderer.UpdateUnits
	}
	cfg.World.OnMapChunkRemoved = func(origin util.DFCoord) {
		cfg.Mesher.RemoveChunk(origin)
		if cfg.Renderer != nil {
			cfg.Renderer.RemoveChunk(origin)
		}
	}

	// 3. Conexão Mundo -> HUD e Câmera (Estado Global e Inicial)
	hasTeleported := false
	cfg.World.OnWorldStatus = func(s *fvnet.WorldStatus) {
		// Teleporte inicial para o centro da fortaleza se ainda não o fizemos
		if !hasTeleported && s.ViewX > 0 {
			cfg.Camera.Teleport(float32(s.ViewX), float32(s.ViewY), float32(s.ViewZ))
			fmt.Printf("[Bridge] 🚀 Câmera posicionada no centro da visão: (%d, %d, %d)\n", s.ViewX, s.ViewY, s.ViewZ)
			hasTeleported = true
		}

		// Atualizar elementos da interface
		cfg.HUD.UpdateWorld(s.WorldName)
		cfg.HUD.UpdatePop(int(s.Population))
		cfg.HUD.UpdateSeason(fmt.Sprintf("Ano %d", s.Year))
		cfg.HUD.SetZOffset(s.ZOffset)
	}

	// 4. Conexão Mundo -> Logs/Status
	cfg.World.OnStatusMsg = func(msg string, dfConnected bool) {
		fmt.Printf("[Bridge] 📡 Status: %s (DF: %v)\n", msg, dfConnected)
	}
}
