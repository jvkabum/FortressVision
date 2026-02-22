package app

import (
	"FortressVision/internal/util"
	"fmt"
	"log"
	"time"
)

// MapScanner realiza a varredura do mapa em background para preencher o MapDataStore.
type MapScanner struct {
	app *App
}

func NewMapScanner(app *App) *MapScanner {
	return &MapScanner{app: app}
}

// Start inicia a goroutine de varredura normal (streaming).
func (s *MapScanner) Start() {
	go s.scanLoop()
}

// StartFullScan inicia o download completo do mundo.
func (s *MapScanner) StartFullScan() {
	go s.fullScanLoop()
}

func (s *MapScanner) scanLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro no MapScanner: %v", r)
		}
	}()
	log.Printf("[Scanner] Aguardando inicialização do App...")
	for s.app.State != StateViewing {
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[Scanner] Iniciando varredura total do mapa (Prioridade: Nível de Foco)...")

	for {
		if s.app.dfClient == nil || !s.app.dfClient.IsConnected() {
			time.Sleep(2 * time.Second)
			continue
		}

		info := s.app.dfClient.MapInfo
		if info == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Nova estratégia: Foco central (0), TODOS para cima, e 30 para baixo (prioritário).
		zOffsets := []int32{0}
		// Níveis acima (todos)
		for i := int32(1); i <= 100; i++ {
			zOffsets = append(zOffsets, i)
		}
		// Níveis abaixo (30 prioritários - cobre quase qualquer montanha/vale visível)
		for i := int32(1); i <= 30; i++ {
			zOffsets = append(zOffsets, -i)
		}
		// Resto dos níveis abaixo (baixa prioridade)
		for i := int32(31); i <= 60; i++ {
			zOffsets = append(zOffsets, -i)
		}

		radius := int32(128) // 8 blocos

		for _, offset := range zOffsets {
			center := s.app.mapCenter
			z := center.Z + offset

			log.Printf("[Scanner] Varrendo nível Z=%d (Offset %d)", z, offset)

			// Varre em blocos de 32x32 para eficiência
			for x := center.X - radius; x < center.X+radius; x += 16 {
				for y := center.Y - radius; y < center.Y+radius; y += 16 {
					minX, maxX := x, x+15
					minY, maxY := y, y+15
					// CRUCIAL: Alinhar ao grid de 16x16 para bater com as chaves do banco de dados/cache
					origin := util.DFCoord{X: minX, Y: minY, Z: z}.BlockCoord()

					// Verifica se o bloco já existe no cache (SQL ou visita anterior)
					s.app.mapStore.Mu.RLock()
					_, exists := s.app.mapStore.Chunks[origin]
					s.app.mapStore.Mu.RUnlock()

					// Se o bloco já existe na memória (populada pelo SQL no boot ou por visita anterior),
					// evitamos pedir ao DFHack e reportamos o uso do cache local.
					if exists {
						log.Printf("[Scanner] SQL-Cache Hit: %s (Reutilizando dados do Banco)", origin)
						continue
					}

					list, err := s.app.dfClient.GetBlockList(minX, minY, z, maxX, maxY, z, 256)
					if err != nil {
						// Se falhar (ex: gRPC busy), espera um pouco e pula o bloco
						time.Sleep(100 * time.Millisecond)
						continue
					}

					for _, block := range list.MapBlocks {
						s.app.mapStore.StoreSingleBlock(&block)
					}

					// Pausa maior para manter o gRPC saudável e FPS alto
					time.Sleep(50 * time.Millisecond)
				}
			}
			// Pausa entre níveis Z
			time.Sleep(200 * time.Millisecond)
		}

		// Espera longa antes do próximo ciclo de "refresh" total
		time.Sleep(30 * time.Second)
	}
}
func (s *MapScanner) fullScanLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro no FullScan: %v", r)
		}
	}()

	log.Printf("[Scanner] Iniciando download TOTAL do mapa...")

	for s.app.dfClient == nil || !s.app.dfClient.IsConnected() {
		time.Sleep(500 * time.Millisecond)
	}

	info := s.app.dfClient.MapInfo
	if info == nil {
		return
	}

	// Estratégia Armok Vision: DF pode ter até 256+ níveis Z.
	// As coordenadas Z podem ser negativas (bedrock comumente em -126).
	totalX := info.BlockSizeX
	totalY := info.BlockSizeY
	const minZ = -130
	const maxZ = 200
	const totalSteps = maxZ - minZ

	s.app.LoadingTotalBlocks = int(totalSteps)
	s.app.FullScanActive = true

	for z := int32(minZ); z < int32(maxZ); z++ {
		// Se o usuário pular ou fechar, paramos
		if !s.app.Loading || !s.app.FullScanActive {
			break
		}

		processed := z - minZ
		s.app.LoadingStatus = fmt.Sprintf("Baixando Mundo: Camada Z %d [%d/%d]", z, processed, totalSteps)
		s.app.LoadingProcessedBlocks = int(processed)
		s.app.LoadingProgress = float32(processed) / float32(totalSteps)

		// Varre o nível Z em lotes de 128x128 para velocidade máxima gRPC
		for x := int32(0); x < totalX; x += 128 {
			for y := int32(0); y < totalY; y += 128 {
				maxX := x + 127
				if maxX >= totalX {
					maxX = totalX - 1
				}
				maxY := y + 127
				if maxY >= totalY {
					maxY = totalY - 1
				}

				list, err := s.app.dfClient.GetBlockList(x, y, z, maxX, maxY, z, 10000) // Lote maior
				if err != nil {
					time.Sleep(10 * time.Millisecond) // Erro rápido
					continue
				}

				for _, block := range list.MapBlocks {
					s.app.mapStore.StoreSingleBlock(&block)
				}
				if len(list.MapBlocks) > 0 {
					log.Printf("[Scanner] Camada Z %d: %d blocos recebidos", z, len(list.MapBlocks))
				}
				// Sem sleep aqui para velocidade máxima
			}
		}
	}

	log.Println("[Scanner] Download total concluído!")
	s.app.Loading = false
	s.app.FullScanActive = false
	s.app.State = StateViewing

	// Inicia o scanner de streaming normal após o FullScan
	s.Start()
}
