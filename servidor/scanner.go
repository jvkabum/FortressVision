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
	log.Println("[Scanner] Iniciando loop de varredura prioritária do Servidor...")

	for {
		if !s.dfClient.IsConnected() {
			time.Sleep(2 * time.Second)
			continue
		}

		info := s.dfClient.MapInfo
		if info == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Estratégia do usuário: Foco central (0), cima (+100) e baixo (-60)
		zOffsets := []int32{0}
		for i := int32(1); i <= 100; i++ {
			zOffsets = append(zOffsets, i)
		}
		for i := int32(1); i <= 30; i++ {
			zOffsets = append(zOffsets, -i)
		}
		for i := int32(31); i <= 60; i++ {
			zOffsets = append(zOffsets, -i)
		}

		radius := int32(128)
		view, err := s.dfClient.GetViewInfo()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		center := util.DFCoord{X: view.ViewPosX, Y: view.ViewPosY, Z: view.ViewPosZ}

		for _, offset := range zOffsets {
			z := center.Z + offset

			for x := center.X - radius; x < center.X+radius; x += 16 {
				for y := center.Y - radius; y < center.Y+radius; y += 16 {
					minX, maxX := x, x+15
					minY, maxY := y, y+15

					list, err := s.dfClient.GetBlockList(minX, minY, z, maxX, maxY, z, 256)
					if err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					for _, block := range list.MapBlocks {
						s.store.StoreSingleBlock(&block)
					}
					time.Sleep(50 * time.Millisecond)
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(30 * time.Second)
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
