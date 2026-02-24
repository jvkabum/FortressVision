package main

import (
	"FortressVision/servidor/internal/dfhack"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"log"
	"time"
)

type ServerScanner struct {
	dfClient *dfhack.Client
	store    *mapdata.MapDataStore
	hub      *Hub
}

func NewServerScanner(df *dfhack.Client, s *mapdata.MapDataStore, h *Hub) *ServerScanner {
	return &ServerScanner{
		dfClient: df,
		store:    s,
		hub:      h,
	}
}

func (s *ServerScanner) Start() {
	go s.scanLoop()
}

func (s *ServerScanner) scanLoop() {
	log.Println("[Scanner] Iniciando loop de varredura ultra-rápida do Servidor...")

	for {
		if !s.dfClient.IsConnected() {
			time.Sleep(2 * time.Second)
			continue
		}

		interestZ := s.dfClient.GetInterestZ()
		radius := int32(192) // Expandido para 384x384 (24x24 blocos)
		view, err := s.dfClient.GetViewInfo()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
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
			// log.Printf("[Scanner] Trace: Iniciando varredura da camada Z=%d", z)
			// Verifica se o foco do jogador mudou drasticamente a cada camada
			if currentZ := s.dfClient.GetInterestZ(); util.Abs(currentZ-interestZ) > 3 {
				log.Printf("[Scanner] Foco mudou (Z:%d -> Z:%d). Reiniciando varredura.", interestZ, currentZ)
				break // Sai do loop de camadas para pegar o novo centro
			}

			// Busca TODA a camada de interesse em uma única chamada (Alta Performance)
			minX, maxX := center.X-radius, center.X+radius
			minY, maxY := center.Y-radius, center.Y+radius

			// Pedimos até 600 blocos (24x24 = 576 blocos)
			list, err := s.dfClient.GetBlockList(minX, minY, z, maxX, maxY, z, 600)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			blocksUpdated := 0
			for _, block := range list.MapBlocks {
				change := s.store.StoreSingleBlock(&block)
				if change != mapdata.NoChange {
					blocksUpdated++
					if change == mapdata.VegetationChange {
						chunkOrigin := util.NewDFCoord(block.MapX, block.MapY, block.MapZ).BlockCoord()
						chunk := s.store.Chunks[chunkOrigin]
						s.hub.BroadcastVegetation(block.MapX, block.MapY, block.MapZ, chunk.Plants)
					}
				}
			}

			if blocksUpdated > 0 {
				log.Printf("[Scanner] Camada Z %d: %d blocos novos/atualizados.", z, blocksUpdated)
			}

			// Pequeno fôlego para o DFHack
			time.Sleep(40 * time.Millisecond)
		}

		// Pausa antes do próximo ciclo completo
		time.Sleep(40 * time.Millisecond)
	}
}

func (s *ServerScanner) StartFullScan() {
	go func() {
		log.Printf("[Scanner] Iniciando download TOTAL do mapa no servidor...")
		for !s.dfClient.IsConnected() {
			time.Sleep(500 * time.Millisecond)
		}

		info := s.dfClient.MapInfo
		if info == nil {
			return
		}

		const minZ, maxZ = -130, 200
		totalX, totalY := info.BlockSizeX, info.BlockSizeY

		for z := int32(minZ); z < int32(maxZ); z++ {
			for x := int32(0); x < totalX; x += 128 {
				for y := int32(0); y < totalY; y += 128 {
					maxX, maxY := x+127, y+127
					if maxX >= totalX {
						maxX = totalX - 1
					}
					if maxY >= totalY {
						maxY = totalY - 1
					}

					list, err := s.dfClient.GetBlockList(x, y, z, maxX, maxY, z, 10000)
					if err == nil {
						for _, block := range list.MapBlocks {
							s.store.StoreSingleBlock(&block)
						}
					}
				}
			}
			log.Printf("[Scanner] Camada Z %d concluída no servidor", z)
		}
		log.Println("[Scanner] Download total concluído no servidor!")
	}()
}
