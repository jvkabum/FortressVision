package app

import (
	"fmt"
	"log"

	"FortressVision/cliente/internal/client"
	"FortressVision/cliente/internal/meshing"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

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
		// Detecta progresso de varredura total
		if len(msg) > 10 && msg[:10] == "FULL_SCAN:" {
			// Resetamos o timer de failsafe do loading sempre que houver progresso
			a.LoadingStartTime = rl.GetTime()

			status := msg[10:]
			if status == "DONE" {
				a.FullScanActive = false
				a.LoadingStatus = "Mundo sincronizado!"
			} else {
				a.FullScanActive = true
				var current, total int
				fmt.Sscanf(status, "%d/%d", &current, &total)
				if total > 0 {
					a.LoadingProgress = float32(current) / float32(total)
					a.LoadingStatus = fmt.Sprintf("Primeiro arranque do mundo. Isso pode levar alguns minutos...\nVarredura: Nível Z %d de %d (%.1f%%)", current, total, a.LoadingProgress*100)
				}
			}
			return
		}
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
				MaxDepth: 48,
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
		a.WorldPopulation = int(status.GetPopulation())
		a.ZOffset = status.GetZOffset()

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
