package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"log" // Added based on the provided Code Edit
	"sync"
)

// BlockMesher implementa o gerador de malha para blocos do mapa.
type BlockMesher struct {
	requests    chan Request
	results     chan Result
	stop        chan struct{}
	MatStore    *mapdata.MaterialStore
	ResultStore *ResultStore
	pending     map[util.DFCoord]bool
	pendingMu   sync.Mutex
}

// NewBlockMesher cria e inicia um novo mesher.
func NewBlockMesher(workers int, matStore *mapdata.MaterialStore, resultStore *ResultStore) *BlockMesher {
	m := &BlockMesher{
		requests:    make(chan Request, 2000),
		results:     make(chan Result, 2000),
		stop:        make(chan struct{}),
		MatStore:    matStore,
		ResultStore: resultStore,
		pending:     make(map[util.DFCoord]bool),
	}

	for i := 0; i < workers; i++ {
		go m.worker()
	}

	return m
}

func (m *BlockMesher) Enqueue(req Request) bool {
	m.pendingMu.Lock()
	if m.pending[req.Origin] {
		m.pendingMu.Unlock()
		return false
	}
	m.pending[req.Origin] = true
	m.pendingMu.Unlock()

	select {
	case m.requests <- req:
		return true
	default:
		// Se a fila estiver cheia, remove do pendente para tentar depois
		m.pendingMu.Lock()
		delete(m.pending, req.Origin)
		m.pendingMu.Unlock()
		return false
	}
}

func (m *BlockMesher) Results() <-chan Result {
	return m.results
}

func (m *BlockMesher) Stop() {
	close(m.stop)
}

func (m *BlockMesher) worker() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro no Mesher Worker: %v", r)
		}
	}()
	for {
		select {
		case req := <-m.requests:
			// 1. Verificar Cache antes de processar
			if m.ResultStore != nil {
				if cached, ok := m.ResultStore.Get(req.Origin, req.MTime); ok {
					m.pendingMu.Lock()
					delete(m.pending, req.Origin)
					m.pendingMu.Unlock()
					m.results <- cached
					continue
				}
			}

			// 2. Gerar geometria se não estiver no cache
			res := m.Generate(req)

			// 3. Salvar no cache para uso futuro
			if m.ResultStore != nil {
				m.ResultStore.Store(res)
			}

			m.pendingMu.Lock()
			delete(m.pending, req.Origin)
			m.pendingMu.Unlock()
			m.results <- res
		case <-m.stop:
			return
		}
	}
}

// Generate transforma um bloco de tiles em geometria.
func (m *BlockMesher) Generate(req Request) Result {
	res := Result{
		Origin:             req.Origin,
		MTime:              req.MTime,
		MaterialGeometries: make(map[string]GeometryData),
	}

	// Buffers temporários por textura
	textureBuffers := make(map[string]*MeshBuffer)
	liquidBuffer := GetMeshBuffer()

	// Função auxiliar para obter ou criar buffer para uma textura
	getBuffer := func(name string) *MeshBuffer {
		if buf, ok := textureBuffers[name]; ok {
			return buf
		}
		buf := GetMeshBuffer()
		textureBuffers[name] = buf
		return buf
	}

	// Algoritmo Greedy Meshing 2D (Otimização Massiva - Fase 32)
	m.runGreedyMesher2D(req, getBuffer, liquidBuffer, &res)

	// Converter buffers para GeometryData
	for name, buf := range textureBuffers {
		if len(buf.Geometry.Vertices) > 0 {
			res.MaterialGeometries[name] = buf.Geometry.Clone()
		}
		PutMeshBuffer(buf)
	}

	res.Liquidos = liquidBuffer.Geometry.Clone()
	PutMeshBuffer(liquidBuffer)

	return res
}

func (m *BlockMesher) runGreedyMesher2D(req Request, getBuffer func(string) *MeshBuffer, liquidBuffer *MeshBuffer, res *Result) {
	currentZ := req.Origin.Z

	// Lista das 6 direções principais para o Greedy Meshing
	coreDirs := []util.Directions{
		util.DirUp, util.DirDown,
		util.DirNorth, util.DirSouth,
		util.DirWest, util.DirEast,
	}

	// Para cada uma das 6 faces, executamos a otimização
	for _, faceDir := range coreDirs {
		// 1. Gerar Máscara 16x16 para esta face
		// Usamos uint32 para codificar [MaterialID(16 bits) | Color(16 bits reduzido)]
		mask := make([]uint32, 16*16)
		tiles := make([]*mapdata.Tile, 16*16)

		for yy := int32(0); yy < 16; yy++ {
			for xx := int32(0); xx < 16; xx++ {
				worldCoord := util.NewDFCoord(req.Origin.X+xx, req.Origin.Y+yy, currentZ)
				tile := req.Data.GetTile(worldCoord)

				if tile == nil || tile.Hidden || tile.Shape() == dfproto.ShapeNoShape {
					continue
				}

				// Líquidos e Objetos Especiais (Rampas/Escadas) não entram no Greedy Meshing Comum
				// Líquidos são processados apenas uma vez (na primeira face iterada)
				// Líquidos e Objetos Especiais (Rampas/Escadas/Vegetação)
				// Processamos modelos 3D apenas UMA VEZ por tile (escolhemos DirUp arbitrariamente)
				if faceDir == util.DirUp {
					GenerateLiquidGeometry(tile, liquidBuffer)

					shape := tile.Shape()
					if shape == dfproto.ShapeRamp {
						rlColor := m.MatStore.GetTileColor(tile)
						m.addRamp(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, res)
						continue
					}
					if shape == dfproto.ShapeStairUp || shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
						texName := m.MatStore.GetTextureName(tile.MaterialCategory())
						rlColor := m.MatStore.GetTileColor(tile)
						m.addStairs(worldCoord, shape, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer(texName), req.Data)
						continue
					}
					// Árvores e vegetação 3D (Lógica Armok Vision Refinada)
					if shape == dfproto.ShapeTrunkBranch {
						rlColor := m.MatStore.GetTileColor(tile)
						// Tronco: usamos o modelo sólido (pillar)
						// addTrunk agora usará "tree_trunk" (pillar) para o corpo e TREE.obj apenas na base se necessário
						m.addTrunk(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer, res, req.Data)
						continue
					}
					if shape == dfproto.ShapeTreeShape {
						// Copa: volumosa mas leve. Em vez do TREE.obj massivo em cada bloco, usamos raminhos esparsos.
						// Estratégia: Desenha 1 em cada 4 tiles (padrão xadrez 3D esparso)
						if (worldCoord.X%2 == 0) && (worldCoord.Y%2 == 0) && (worldCoord.Z%2 != 0) {
							rlColor := m.MatStore.GetTileColor(tile)
							m.addTwig(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer, res, req.Data)
						}
						continue
					}
					if shape == dfproto.ShapeBranch {
						rlColor := m.MatStore.GetTileColor(tile)
						m.addBranch(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer, res, req.Data)
						continue
					}
					if shape == dfproto.ShapeTwig {
						rlColor := m.MatStore.GetTileColor(tile)
						m.addTwig(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer, res, req.Data)
						continue
					}
					if shape == dfproto.ShapeSapling || shape == dfproto.ShapeShrub {
						rlColor := m.MatStore.GetTileColor(tile)
						m.addShrub(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, res)
						// Solo sob arbusto será gerado pelo greedy mesher se for floor
					}
					// Gramas (Baseado no GrassPercent)
					if tile.GrassPercent > 0 {
						m.addGrass(worldCoord, tile, res)
					}
				}

				// Se a face deve ser desenhada, adicionamos à máscara
				if m.shouldDrawFace(tile, faceDir) {
					// Pular formas que não são cubos padrão
					shape := tile.Shape()
					if shape != dfproto.ShapeWall && shape != dfproto.ShapeFloor && shape != dfproto.ShapeFortification {
						continue
					}

					texID := uint32(tile.MaterialCategory())
					rlColor := m.MatStore.GetTileColor(tile)
					colorID := uint32(rlColor.R)<<16 | uint32(rlColor.G)<<8 | uint32(rlColor.B)

					// Oclusão Ambiente (AO): Para fundir faces, elas devem ter o mesmo padrão de AO nos cantos.
					// Isso evita interpolação errada de sombras em quads grandes.
					aoMask := m.calculateAOMask(worldCoord, faceDir, req.Data)

					mask[yy*16+xx] = (texID << 24) | (aoMask << 20) | (colorID & 0xFFFFF) | 1<<31
					tiles[yy*16+xx] = tile
				}
			}
		}

		// 2. Resolver a máscara (Otimização 2D)
		for y := int32(0); y < 16; y++ {
			for x := int32(0); x < 16; x++ {
				idx := y*16 + x
				if mask[idx] == 0 {
					continue
				}

				currentMaskVal := mask[idx]
				currentTile := tiles[idx]

				// Encontrar largura máxima (W)
				var w int32 = 1
				canGrowW := true
				if faceDir == util.DirWest || faceDir == util.DirEast {
					canGrowW = false
				}

				if canGrowW {
					for x+w < 16 && mask[y*16+(x+w)] == currentMaskVal {
						w++
					}
				}

				// Encontrar altura máxima (H) para essa largura
				var h int32 = 1
				// Restrição Crítica: Faces Laterais não podem ser fundidas no eixo de profundidade
				canGrowH := true
				if faceDir == util.DirNorth || faceDir == util.DirSouth {
					canGrowH = false
				}

				if canGrowH {
				loopH:
					for y+h < 16 {
						for k := int32(0); k < w; k++ {
							if mask[(y+h)*16+(x+k)] != currentMaskVal {
								break loopH
							}
						}
						h++
					}
				}

				// Gerar o Quad (Retângulo otimizado)
				m.emitGreedyQuad(req, x, y, w, h, faceDir, currentTile, getBuffer)

				// Limpar a área processada na máscara
				for jh := int32(0); jh < h; jh++ {
					for jw := int32(0); jw < w; jw++ {
						mask[(y+jh)*16+(x+jw)] = 0
					}
				}
			}
		}
	}
}
