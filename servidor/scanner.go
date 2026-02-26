package main

import (
	"FortressVision/servidor/internal/dfhack"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"
	"log"
	"sync"
	"time"
)

type ServerScanner struct {
	dfClient *dfhack.Client
	store    *mapdata.MapDataStore
	hub      *Hub

	// Evita escanear o mesmo Z-Level repetidas vezes simultaneamente
	zLevelLocks sync.Map

	// Flag para controlar a suspensão de rotinas menores durante scans intensos
	isFullScanning bool
	fsMutex        sync.RWMutex
}

func NewServerScanner(df *dfhack.Client, s *mapdata.MapDataStore, h *Hub) *ServerScanner {
	return &ServerScanner{
		dfClient:       df,
		store:          s,
		hub:            h,
		isFullScanning: false,
	}
}

func (s *ServerScanner) Start() {
	go s.scanLoop()
}

func (s *ServerScanner) scanLoop() {
	log.Println("[Scanner] Iniciando loop de varredura ultra-rápida do Servidor...")

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Scanner-Loop] Recuperado de pânico: %v", r)
				}
			}()

			// O scanner direcional agora pode rodar em paralelo ao Full Scan (Fase 8)
			// Isso garante que o nível Z onde o jogador está olhando seja priorizado/atualizado.

			if !s.dfClient.IsConnected() {
				time.Sleep(2 * time.Second)
				return
			}

			interestZ := s.dfClient.GetInterestZ()
			radius := int32(192) // Expandido para 384x384 (24x24 blocos)
			view, err := s.dfClient.GetViewInfo()
			if err != nil || view == nil {
				time.Sleep(1 * time.Second)
				return
			}

			// Ordem de prioridade em espiral (0, -1, 1, -2, 2...)
			zOffsets := []int32{0}
			for i := int32(1); i <= 80; i++ {
				zOffsets = append(zOffsets, -i)
				zOffsets = append(zOffsets, i)
			}

			center := util.DFCoord{X: view.ViewPosX, Y: view.ViewPosY, Z: interestZ}

			for _, offset := range zOffsets {
				z := center.Z + offset
				if currentZ := s.dfClient.GetInterestZ(); util.Abs(currentZ-interestZ) > 3 {
					log.Printf("[Scanner] Foco mudou (Z:%d -> Z:%d). Reiniciando varredura.", interestZ, currentZ)
					go s.ScanZLevelBackground(currentZ)
					break
				}

				// Limites baseados no MapInfo local (0 a Size-1)
				info := s.dfClient.MapInfo
				if info == nil {
					break
				}

				// NOTA: Agora usamos coordenadas ABSOLUTAS do mundo para tudo.
				bxMin := util.Max(info.BlockPosX*16, center.X-radius)
				bxMax := util.Min((info.BlockPosX+info.BlockSizeX)*16-1, center.X+radius)
				byMin := util.Max(info.BlockPosY*16, center.Y-radius)
				byMax := util.Min((info.BlockPosY+info.BlockSizeY)*16-1, center.Y+radius)

				// Pedimos a região em uma única chamada.
				list, err := s.dfClient.GetBlockList(bxMin/16, byMin/16, z, bxMax/16, byMax/16, z, 500)
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				}

				blocksUpdated := 0
				foundBlockMap := make(map[util.DFCoord]bool)

				if list != nil && len(list.MapBlocks) > 0 {
					for _, block := range list.MapBlocks {
						origin := util.NewDFCoord(block.MapX, block.MapY, block.MapZ).BlockCoord()
						foundBlockMap[origin] = true

						change := s.store.StoreSingleBlock(&block)
						if change != mapdata.NoChange {
							blocksUpdated++
							if change == mapdata.VegetationChange {
								if chunk, ok := s.store.GetChunk(origin); ok {
									// Passar uma cópia do slice para evitar race condition
									plantsCopy := append([]dfproto.PlantDetail(nil), chunk.Plants...)
									s.hub.BroadcastVegetation(block.MapX, block.MapY, block.MapZ, plantsCopy)
								}
							}
						}
					}
				}

				// Marca como vazio o que pedimos e não veio (Ar/Céu)
				for bx := (bxMin / 16) * 16; bx <= bxMax; bx += 16 {
					for by := (byMin / 16) * 16; by <= byMax; by += 16 {
						origin := util.NewDFCoord(bx, by, z).BlockCoord()
						if !foundBlockMap[origin] {
							s.store.MarkAsEmpty(origin)
						}
					}
				}

				if blocksUpdated > 0 {
					log.Printf("[Scanner] Camada Z %d: %d blocos novos/atualizados.", z, blocksUpdated)
				}
				time.Sleep(40 * time.Millisecond)
			}
		}()
		time.Sleep(40 * time.Millisecond)
	}
}

func (s *ServerScanner) StartFullScan() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Scanner-FullScan] Recuperado de pânico fatal: %v", r)
				s.fsMutex.Lock()
				s.isFullScanning = false
				s.fsMutex.Unlock()
			}
		}()
		log.Printf("[Scanner] Iniciando download TOTAL do mapa no servidor...")
		for !s.dfClient.IsConnected() {
			time.Sleep(500 * time.Millisecond)
		}

		info := s.dfClient.MapInfo
		if info == nil {
			return
		}

		// X e Y começam em 0 (Locais), Z é absoluto
		minX, minY, minZ := info.BlockPosX, info.BlockPosY, info.BlockPosZ
		totalBlocksX, totalBlocksY, totalBlocksZ := info.BlockSizeX, info.BlockSizeY, info.BlockSizeZ
		maxZ := minZ + totalBlocksZ

		worldName := s.dfClient.MapInfo.WorldNameEn
		if worldName == "" {
			worldName = s.dfClient.MapInfo.WorldName
		}

		// Travar o scan ultra-rápido (evita competição inútil)
		s.fsMutex.Lock()
		s.isFullScanning = true
		s.fsMutex.Unlock()

		// Destravar no final de forma garantida
		defer func() {
			s.fsMutex.Lock()
			s.isFullScanning = false
			s.fsMutex.Unlock()
			s.hub.BroadcastServerStatus("FULL_SCAN:DONE", s.dfClient.IsConnected())
			log.Println("[Scanner] Download total concluído de forma otimizada!")
		}()

		log.Printf("[Scanner] Iniciando varredura TOTAL linear (Top-Down): %d níveis (Z: %d a %d)", totalBlocksZ, minZ, maxZ-1)

		for z := maxZ - 1; z >= minZ; z-- {
			currentLevel := maxZ - z
			progressMsg := fmt.Sprintf("FULL_SCAN:%d/%d", currentLevel, totalBlocksZ)
			s.hub.BroadcastServerStatus(progressMsg, s.dfClient.IsConnected())

			blocksInLayer := 0
			emptyInLayer := 0

			for x := int32(0); x < totalBlocksX; x += 3 {
				for y := int32(0); y < totalBlocksY; y += 3 {
					maxX, maxY := x+3, y+3
					if maxX > totalBlocksX {
						maxX = totalBlocksX
					}
					if maxY > totalBlocksY {
						maxY = totalBlocksY
					}

					// GetBlockList agora aceita coordenadas GLOBAIS e cuida da tradução interna
					list, err := s.dfClient.GetBlockList(minX+x, minY+y, z, minX+maxX, minY+maxY, z+1, 10)
					if err != nil {
						continue
					}

					foundInBatch := make(map[util.DFCoord]bool)
					if list != nil && len(list.MapBlocks) > 0 {
						for _, block := range list.MapBlocks {
							s.store.StoreSingleBlock(&block)
							blocksInLayer++
							foundInBatch[util.NewDFCoord(block.MapX*16, block.MapY*16, block.MapZ).BlockCoord()] = true
						}
					}

					// Marcar como vazio apenas os blocos globais que tentamos buscar
					for bx := int32(0); bx < (maxX - x); bx++ {
						for by := int32(0); by < (maxY - y); by++ {
							absBX, absBY := minX+x+bx, minY+y+by
							origin := util.DFCoord{X: absBX * 16, Y: absBY * 16, Z: z}
							if !foundInBatch[origin] {
								s.store.MarkAsEmpty(origin)
								emptyInLayer++
							}
						}
					}
					// Acelera as pausas para injetar mais velocidade de rede (era 2ms)
					// time.Sleep(1 * time.Millisecond) // Opcional
				}
			}

			nDone := maxZ - z
			if z%20 == 0 {
				pct := float64(nDone) / float64(totalBlocksZ) * 100
				log.Printf("[Scanner] ████ Progresso: %d/%d níveis (%.0f%%)", nDone, totalBlocksZ, pct)
			}

			// ---> OTIMIZAÇÃO DE MEMÓRIA CRÍTICA (Evitar leak de 18GB) <---
			// Salva o andar que acabamos de receber e força a limpeza da RAM
			savedCount, _ := s.store.Save(worldName)

			// Fazer o purge apenas deste Z-Level que acabamos de carregar
			s.store.Mu.Lock()
			countPurged := 0
			for origin := range s.store.Chunks {
				if origin.Z == z {
					delete(s.store.Chunks, origin)
					countPurged++
				}
			}
			s.store.Mu.Unlock()

			// Log compacto: 1 linha com tudo
			if savedCount > 0 || countPurged > 0 {
				log.Printf("[Scanner] Z=%d ✓ %d terreno, %d céu | %d salvos, %d liberados (%d/%d)", z, blocksInLayer, emptyInLayer, savedCount, countPurged, nDone, totalBlocksZ)
			}

		}
	}()
}

func (s *ServerScanner) ScanZLevelBackground(z int32) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Scanner-BackgroundScan] Recuperado de pânico para Z=%d: %v", z, r)
		}
		s.zLevelLocks.Delete(z)
	}()

	if _, loaded := s.zLevelLocks.LoadOrStore(z, true); loaded {
		return
	}

	for !s.dfClient.IsConnected() {
		time.Sleep(500 * time.Millisecond)
		return
	}

	info := s.dfClient.MapInfo
	if info == nil {
		return
	}

	minX, minY := info.BlockPosX, info.BlockPosY
	totalBlocksX, totalBlocksY := info.BlockSizeX, info.BlockSizeY
	blocksCached := 0

	log.Printf("[Scanner-Cache] Iniciando cache massivo preemptivo do andar Z=%d...", z)

	for x := int32(0); x < totalBlocksX; x += 4 {
		for y := int32(0); y < totalBlocksY; y += 4 {
			// Coordenadas ABSOLUTAS
			absX, absY := minX+x, minY+y
			maxX, maxY := absX+3, absY+3
			if maxX >= minX+totalBlocksX {
				maxX = minX + totalBlocksX - 1
			}
			if maxY >= minY+totalBlocksY {
				maxY = minY + totalBlocksY - 1
			}

			// Coordenadas GLOBAIS
			list, err := s.dfClient.GetBlockList(absX, absY, z, maxX+1, maxY+1, z+1, 16)
			if err == nil && list != nil {
				for _, block := range list.MapBlocks {
					s.store.StoreSingleBlock(&block)
					blocksCached++
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	log.Printf("[Scanner-Cache] Andar Z=%d finalizado. %d blocos pré-aquecidos.", z, blocksCached)
}
