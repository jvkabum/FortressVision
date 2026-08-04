package app

import (
	"fmt"
	"log"

	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

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

func (a *App) updateMap(force bool) {
	if a.netClient == nil || !a.netClient.IsConnected() {
		return
	}

	// throttle: requisita a cada 180 frames (3s) normal, ou 90 frames no loading
	checkInterval := 180
	if a.Loading {
		checkInterval = 90
	}
	if !force && a.frameCount%checkInterval != 0 {
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

			// Lógica de progresso do Loading baseada em novos blocos
			if a.Loading && !a.FullScanActive {
				a.LoadingProcessedBlocks++
				// Calcula progresso visual genérico
				if a.LoadingTotalBlocks > 0 {
					a.LoadingProgress = float32(a.LoadingProcessedBlocks) / float32(a.LoadingTotalBlocks)
					a.LoadingStatus = fmt.Sprintf("Construindo terreno: %d/%d (%.1f%%)",
						a.LoadingProcessedBlocks, a.LoadingTotalBlocks, a.LoadingProgress*100)
				}
			}
		default:
			// Se não há mais resultados, mas estamos no loading, verificamos o FailSafe temporal aqui
			if a.Loading {
				timeSinceSync := rl.GetTime() - a.LoadingStartTime
				loadThreshold := float32(0.35) // Ajustado para 35%

				// Condições de término: (Progresso OK OR Tempo limite atingido) AND Não estar em varredura total
				reachedThreshold := a.LoadingTotalBlocks > 0 && float32(a.LoadingProcessedBlocks)/float32(a.LoadingTotalBlocks) >= loadThreshold
				failedTimeout := timeSinceSync > 20.0 // Aumentado para 20s de margem de segurança

				// Só saímos do loading se não houver varredura total ativa
				if !a.FullScanActive && (reachedThreshold || failedTimeout) {
					a.Loading = false
					a.LoadingProgress = 1.0
					log.Printf("[App] Loading concluído! (%d blocos processados em %.1fs). FailSafe: %v",
						a.LoadingProcessedBlocks, timeSinceSync, failedTimeout)
				}
			}
			return
		}
	}
}

// updateWorldStatus agora pode ser alimentado pelo servidor futuramente.
func (a *App) updateWorldStatus() {}
