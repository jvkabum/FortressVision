package core

import (
	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/hud"
	"FortressVision/cliente/internal/mesher"
	"FortressVision/cliente/internal/network"
	"FortressVision/cliente/internal/world"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/proto/fvnet"
	"fmt"
)

// BridgeConfig contém as instâncias necessárias para configurar a orquestração do app.
type BridgeConfig struct {
	Net     *network.Client
	World   *world.Manager
	Mesher  *mesher.Manager
	HUD     *hud.HUD
	Camera  *camera.Controller
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
			cfg.Mesher.RequestMeshUpdate(chunk)
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
	}

	// 4. Conexão Mundo -> Logs/Status
	cfg.World.OnStatusMsg = func(msg string, dfConnected bool) {
		fmt.Printf("[Bridge] 📡 Status: %s (DF: %v)\n", msg, dfConnected)
	}
}
