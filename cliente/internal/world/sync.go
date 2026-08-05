package world

import (
	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/network"
	"FortressVision/shared/config"
	"FortressVision/shared/util"
	"fmt"
	"math"
	"time"
)

const regionRequestInterval = 350 * time.Millisecond

// SyncManager coordena a sincronização entre a posição da câmera 3D e o mundo do DF.
type SyncManager struct {
	world            *Manager
	net              *network.Client
	cam              *camera.Controller
	cfg              *config.Config
	lastRegion       util.DFCoord
	hasRegionRequest bool
	lastRequestAt    time.Time
}

// NewSyncManager inicializa o gerenciador de sincronização de região.
func NewSyncManager(w *Manager, n *network.Client, c *camera.Controller, conf *config.Config) *SyncManager {
	return &SyncManager{
		world: w,
		net:   n,
		cam:   c,
		cfg:   conf,
	}
}

// Update monitora a posição da câmera e solicita atualizações de região se necessário.
func (s *SyncManager) Update() {
	if s.cam == nil || s.world == nil || s.net == nil {
		return
	}
	// A primeira atualização pode ocorrer antes do handshake WebSocket.
	// Só avance lastPos depois que a conexão estiver ativa.
	if !s.net.IsConnected() {
		return
	}

	// Converter posição de foco da câmera para coordenadas DF
	// Kaiju(X, Y, Z) -> DF(X, Z, -Y) -> util.DFCoord
	dfCoord := util.DFCoord{
		X: int32(math.Floor(float64(s.cam.TargetLookAt[0]))),
		Y: int32(math.Floor(float64(-s.cam.TargetLookAt[2]))),
		Z: int32(math.Floor(float64(s.cam.TargetLookAt[1]))),
	}

	// Só solicita se o foco da câmera mudou de bloco
	region := dfCoord.BlockCoord()
	// A região só precisa ser recarregada ao entrar em um novo chunk de 16x16.
	if s.hasRegionRequest && region == s.lastRegion {
		return
	}
	if s.hasRegionRequest && time.Since(s.lastRequestAt) < regionRequestInterval {
		return
	}

	if !s.hasRegionRequest || region != s.lastRegion {
		fmt.Printf("[SyncManager] 🛰️ Solicitando região para DF%v (Raio:%d)\n", dfCoord, s.cfg.DrawRangeSide)
		// DrawRangeSide is configured in chunks; the region API uses tiles.
		radiusTiles := s.cfg.DrawRangeSide * util.BlockSize
		if radiusTiles < util.BlockSize {
			radiusTiles = util.BlockSize
		}
		s.world.RequestRegion(s.net.Send, dfCoord, radiusTiles, s.cfg.DrawRangeDown, s.cfg.DrawRangeUp)
		s.lastRegion = region
		s.hasRegionRequest = true
		s.lastRequestAt = time.Now()
	}
}
