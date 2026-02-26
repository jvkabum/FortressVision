package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"
	"log" // Added based on the provided Code Edit
	"math"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
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
				if faceDir == 0 {
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
					// Árvores e arbustos também são casos especiais
					if shape == dfproto.ShapeTrunkBranch {
						rlColor := m.MatStore.GetTileColor(tile)
						m.addTrunkBranch(worldCoord, tile, [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}, getBuffer, res, req.Data)
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
				}

				// Se a face deve ser desenhada, adicionamos à máscara
				if m.shouldDrawFace(tile, faceDir) {
					// Pular formas que não são cubos padrão
					shape := tile.Shape()
					if shape != dfproto.ShapeWall && shape != dfproto.ShapeFloor && shape != dfproto.ShapeFortification && shape != dfproto.ShapeTreeShape {
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

func (m *BlockMesher) emitGreedyQuad(req Request, x, y, w, h int32, face util.Directions, tile *mapdata.Tile, getBuffer func(string) *MeshBuffer) {
	// Converte coordenadas locais (0-15) + dimensões (W, H) para mundo real
	pos := util.DFToWorldPos(util.DFCoord{X: req.Origin.X + x, Y: req.Origin.Y + y, Z: req.Origin.Z})

	// Espessura e formato baseados no shape
	shape := tile.Shape()
	thickness := float32(1.0)
	if shape == dfproto.ShapeFloor {
		thickness = 0.1
	}

	// Identificar material e cor
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	targetBuffer := getBuffer(texName)
	rlColor := m.MatStore.GetTileColor(tile)
	color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

	// Chamar o gerador de faces especializado (passando dimensões)
	m.addQuadFace(pos, float32(w), float32(h), thickness, face, color, targetBuffer, util.DFCoord{X: req.Origin.X + x, Y: req.Origin.Y + y, Z: req.Origin.Z}, req.Data)
}

func (m *BlockMesher) addQuadFace(pos rl.Vector3, w, h, thickness float32, face util.Directions, color [4]uint8, buffer *MeshBuffer, coord util.DFCoord, data *mapdata.MapDataStore) {
	px, py, pz := pos.X, pos.Y, pos.Z

	// Cores com AO por vértice
	// Recuperamos o padrao de AO para os cantos deste retângulo
	// Nota: Como garantimos no Greedy Mesher que todos os tiles do quad têm o mesmo AO,
	// podemos pegar o AO de qualquer tile do quad (neste caso, o tile na origem 'coord').
	c1, c2, c3, c4 := m.getQuadCornerColors(coord, face, color, data)

	switch face {
	case util.DirUp:
		buffer.AddFaceUVStandard(
			[3]float32{px, py + thickness, pz}, [2]float32{px, -pz}, c1,
			[3]float32{px + w, py + thickness, pz}, [2]float32{px + w, -pz}, c2,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{px + w, -(pz - h)}, c3,
			[3]float32{px, py + thickness, pz - h}, [2]float32{px, -(pz - h)}, c4,
			[3]float32{0, 1, 0},
		)
	case util.DirDown:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{px, -pz}, c1,
			[3]float32{px, py, pz - h}, [2]float32{px, -(pz - h)}, c2,
			[3]float32{px + w, py, pz - h}, [2]float32{px + w, -(pz - h)}, c3,
			[3]float32{px + w, py, pz}, [2]float32{px + w, -pz}, c4,
			[3]float32{0, -1, 0},
		)
	case util.DirNorth:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{px, -py}, c1,
			[3]float32{px + w, py, pz}, [2]float32{px + w, -py}, c2,
			[3]float32{px + w, py + thickness, pz}, [2]float32{px + w, -(py + thickness)}, c3,
			[3]float32{px, py + thickness, pz}, [2]float32{px, -(py + thickness)}, c4,
			[3]float32{0, 0, 1},
		)
	case util.DirSouth:
		buffer.AddFaceUVStandard(
			[3]float32{px + w, py, pz - h}, [2]float32{px + w, -py}, c1,
			[3]float32{px, py, pz - h}, [2]float32{px, -py}, c2,
			[3]float32{px, py + thickness, pz - h}, [2]float32{px, -(py + thickness)}, c3,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{px + w, -(py + thickness)}, c4,
			[3]float32{0, 0, -1},
		)
	case util.DirWest:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz - h}, [2]float32{-pz + h, -py}, c1,
			[3]float32{px, py, pz}, [2]float32{-pz, -py}, c2,
			[3]float32{px, py + thickness, pz}, [2]float32{-pz, -(py + thickness)}, c3,
			[3]float32{px, py + thickness, pz - h}, [2]float32{-pz + h, -(py + thickness)}, c4,
			[3]float32{-1, 0, 0},
		)
	case util.DirEast:
		buffer.AddFaceUVStandard(
			[3]float32{px + w, py, pz}, [2]float32{-pz, -py}, c1,
			[3]float32{px + w, py, pz - h}, [2]float32{-pz + h, -py}, c2,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{-pz + h, -(py + thickness)}, c3,
			[3]float32{px + w, py + thickness, pz}, [2]float32{-pz, -(py + thickness)}, c4,
			[3]float32{1, 0, 0},
		)
	}
}

// calculateAOMask gera um hash de 4 bits representando o estado de oclusão dos cantos.
func (m *BlockMesher) calculateAOMask(coord util.DFCoord, face util.Directions, data *mapdata.MapDataStore) uint32 {
	// Simplificação: Vamos checar se existem blocos sólidos nas direções cardinais da face
	// Para cada face, existem 4 cantos. Cada canto é afetado por 2 vizinhos cardinais.
	// Se ambos forem sólidos, o AO é máximo (0.8).
	var mask uint32
	getBit := func(dir util.Directions) uint32 {
		if m.isSolidAO(coord, dir, data) {
			return 1
		}
		return 0
	}

	switch face {
	case util.DirUp, util.DirDown:
		mask |= getBit(util.DirNorth) << 0
		mask |= getBit(util.DirSouth) << 1
		mask |= getBit(util.DirWest) << 2
		mask |= getBit(util.DirEast) << 3
	case util.DirNorth, util.DirSouth:
		mask |= getBit(util.DirUp) << 0
		mask |= getBit(util.DirDown) << 1
		mask |= getBit(util.DirWest) << 2
		mask |= getBit(util.DirEast) << 3
	case util.DirWest, util.DirEast:
		mask |= getBit(util.DirUp) << 0
		mask |= getBit(util.DirDown) << 1
		mask |= getBit(util.DirNorth) << 2
		mask |= getBit(util.DirSouth) << 3
	}
	return mask
}

func (m *BlockMesher) getQuadCornerColors(coord util.DFCoord, face util.Directions, baseColor [4]uint8, data *mapdata.MapDataStore) (c1, c2, c3, c4 [4]uint8) {
	applyAO := func(c [4]uint8, ao float32) [4]uint8 {
		return [4]uint8{uint8(float32(c[0]) * ao), uint8(float32(c[1]) * ao), uint8(float32(c[2]) * ao), c[3]}
	}

	getAO := func(d1, d2, d3 util.Directions) float32 {
		occ1 := m.isSolidAO(coord, d1, data)
		occ2 := m.isSolidAO(coord, d2, data)
		occ3 := m.isSolidAO(coord, d3, data)
		if occ1 && occ2 {
			return 0.8
		}
		res := 1.0
		if occ1 {
			res -= 0.05
		}
		if occ2 {
			res -= 0.05
		}
		if occ3 {
			res -= 0.05
		}
		return float32(res)
	}

	switch face {
	case util.DirUp:
		c1 = applyAO(baseColor, getAO(util.DirNorth, util.DirWest, util.DirNorthWest))
		c2 = applyAO(baseColor, getAO(util.DirNorth, util.DirEast, util.DirNorthEast))
		c3 = applyAO(baseColor, getAO(util.DirSouth, util.DirEast, util.DirSouthEast))
		c4 = applyAO(baseColor, getAO(util.DirSouth, util.DirWest, util.DirSouthWest))
	default:
		// Fallback para outras faces (AO simplificado ou nenhum por enquanto para manter performance)
		return baseColor, baseColor, baseColor, baseColor
	}
	return
}

func (m *BlockMesher) generateTileGeometry(tile *mapdata.Tile, buffer *MeshBuffer) {
	// Agora substituído pelo GreedyMesher interno
}

// addCubeGreedy adiciona um cubo com culling de faces previamente calculado pelo Greedy Mesher.
func (m *BlockMesher) addCubeGreedy(pos rl.Vector3, w, h, d float32, color [4]uint8,
	drawUp, drawDown, drawNorth, drawSouth, drawWest, drawEast bool, buffer *MeshBuffer, coord util.DFCoord, data *mapdata.MapDataStore) {
	x, y, z := pos.X, pos.Y, pos.Z

	// Helper para Ambient Occlusion (DAO)
	// Retorna um multiplicador de cor baseado na presença de blocos vizinhos
	getAO := func(c util.DFCoord, d1, d2, d3 util.Directions) float32 {
		occ1 := m.isSolidAO(c, d1, data)
		occ2 := m.isSolidAO(c, d2, data)
		occ3 := m.isSolidAO(c, d3, data)

		if occ1 && occ2 {
			return 0.8 // Canto triplo (suave)
		}
		res := 1.0
		if occ1 {
			res -= 0.05
		}
		if occ2 {
			res -= 0.05
		}
		if occ3 {
			res -= 0.05
		}
		return float32(res)
	}

	applyAO := func(c [4]uint8, ao float32) [4]uint8 {
		return [4]uint8{uint8(float32(c[0]) * ao), uint8(float32(c[1]) * ao), uint8(float32(c[2]) * ao), c[3]}
	}

	// Face Topo (+Y)
	if drawUp {
		aoNW := getAO(coord, util.DirNorth, util.DirWest, util.DirNorthWest)
		aoNE := getAO(coord, util.DirNorth, util.DirEast, util.DirNorthEast)
		aoSW := getAO(coord, util.DirSouth, util.DirWest, util.DirSouthWest)
		aoSE := getAO(coord, util.DirSouth, util.DirEast, util.DirSouthEast)

		// Topo usa X e Z (Z no DF é Y no Raylib, mas aqui estamos no Raylib space já)
		// No Raylib: X é X, Y é Y (altura), Z é Z.
		// Queremos a repetição baseada em X e Z do mundo.
		buffer.AddFaceUVStandard(
			[3]float32{x, y + h, z}, [2]float32{x, -z}, applyAO(color, aoNW),
			[3]float32{x + w, y + h, z}, [2]float32{x + w, -z}, applyAO(color, aoNE),
			[3]float32{x + w, y + h, z - d}, [2]float32{x + w, -(z - d)}, applyAO(color, aoSE),
			[3]float32{x, y + h, z - d}, [2]float32{x, -(z - d)}, applyAO(color, aoSW),
			[3]float32{0, 1, 0},
		)
	}

	// Face Baixo (-Y)
	if drawDown {
		buffer.AddFaceUV(
			[3]float32{x, y, z}, [3]float32{x, y, z - d}, [3]float32{x + w, y, z - d}, [3]float32{x + w, y, z},
			[2]float32{x, -z}, [2]float32{x, -(z - d)}, [2]float32{x + w, -(z - d)}, [2]float32{x + w, -z},
			[3]float32{0, -1, 0}, color,
		)
	}

	// Face Norte (+Z)
	if drawNorth {
		buffer.AddFaceUV(
			[3]float32{x, y, z}, [3]float32{x + w, y, z}, [3]float32{x + w, y + h, z}, [3]float32{x, y + h, z},
			[2]float32{x, -y}, [2]float32{x + w, -y}, [2]float32{x + w, -(y + h)}, [2]float32{x, -(y + h)},
			[3]float32{0, 0, 1}, color,
		)
	}

	// Face Sul (-Z)
	if drawSouth {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z - d}, [3]float32{x, y, z - d}, [3]float32{x, y + h, z - d}, [3]float32{x + w, y + h, z - d},
			[2]float32{x + w, -y}, [2]float32{x, -y}, [2]float32{x, -(y + h)}, [2]float32{x + w, -(y + h)},
			[3]float32{0, 0, -1}, color,
		)
	}

	// Face Oeste (-X)
	if drawWest {
		buffer.AddFaceUV(
			[3]float32{x, y, z - d}, [3]float32{x, y, z}, [3]float32{x, y + h, z}, [3]float32{x, y + h, z - d},
			[2]float32{-z + d, -y}, [2]float32{-z, -y}, [2]float32{-z, -(y + h)}, [2]float32{-z + d, -(y + h)},
			[3]float32{-1, 0, 0}, color,
		)
	}

	// Face Leste (+X)
	if drawEast {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z}, [3]float32{x + w, y, z - d}, [3]float32{x + w, y + h, z - d}, [3]float32{x + w, y + h, z},
			[2]float32{-z, -y}, [2]float32{-z + d, -y}, [2]float32{-z + d, -(y + h)}, [2]float32{-z, -(y + h)},
			[3]float32{1, 0, 0}, color,
		)
	}
}

// shouldDrawFace decide se uma face deve ser renderizada.
func (m *BlockMesher) shouldDrawFace(tile *mapdata.Tile, dir util.Directions) bool {
	neighbor := tile.GetNeighbor(dir)

	// Se não há vizinho carregado, desenha (borda do chunk carragado)
	if neighbor == nil {
		return true
	}

	// Se o vizinho está escondido, desenha
	if neighbor.Hidden {
		return true
	}

	neighborShape := neighbor.Shape()
	if neighborShape == dfproto.ShapeNoShape {
		return true
	}

	// Se eu sou parede e o vizinho é parede, não desenha face interna
	if (tile.Shape() == dfproto.ShapeWall || tile.Shape() == dfproto.ShapeFortification) &&
		(neighborShape == dfproto.ShapeWall || neighborShape == dfproto.ShapeFortification) {
		// Se ambos forem fortificações ou um for fortificação, só não desenha se as faces forem totalmente sólidas.
		// Mas como fortificações têm fendas, é melhor SEMPRE desenhar as faces laterais da fenda se o vizinho for uma wall.
		// Porém, para otimização, se forem duas fortificações adjacentes, as fendas se alinham.
		if tile.Shape() == dfproto.ShapeWall && neighborShape == dfproto.ShapeWall {
			return false
		}
		// Se houver uma fortificação, permitimos o culling apenas se for Top/Bottom, onde são sólidas.
		if dir == util.DirUp || dir == util.DirDown {
			return false
		}
		// Para as laterais, desenha para garantir que a fenda não mostre o "nada" interno do bloco adjacente
		return true
	}

	// Caso especial: Piso em cima de parede sólida
	if dir == util.DirDown && tile.Shape() == dfproto.ShapeFloor && neighborShape == dfproto.ShapeWall {
		return false
	}

	return true
}

func (m *BlockMesher) addRamp(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)

	// Garante que o tipo de rampa foi calculado usando a lógica de 8 vizinhos do Armok
	tile.CalculateRampType()
	rampType := tile.RampType

	// Se não houver rampa válida (ex: isolada), o Armok as vezes retorna 0 ou usa um valor padrão.
	// Vamos usar a rampa 1 como fallback se o tipo for inválido.
	if rampType <= 0 || rampType > 26 {
		rampType = 1
	}

	modelName := fmt.Sprintf("ramp_%d", rampType)

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y, pos.Z - 0.5},
		Scale:       1.0,
		Color:       color,
	})
}

func (m *BlockMesher) isSolidAO(coord util.DFCoord, dir util.Directions, data *mapdata.MapDataStore) bool {
	neighborPos := coord.AddDir(dir)
	tile := data.GetTile(neighborPos)
	if tile == nil {
		return false
	}
	shape := tile.Shape()
	return shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification
}

func (m *BlockMesher) isSolid(coord util.DFCoord, dir util.Directions, data *mapdata.MapDataStore) bool {
	neighborCoord := coord.Add(util.DirOffsets[dir])
	tile := data.GetTile(neighborCoord)
	if tile == nil {
		return false
	}
	shape := tile.Shape()
	return shape == dfproto.ShapeWall || shape == dfproto.ShapeRamp
}

func (m *BlockMesher) addStairs(coord util.DFCoord, shape dfproto.TiletypeShape, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Desenha 4 degraus subindo
	// O DF não especifica a direção das escadas, então vamos fazer uma direção fixa por enquanto
	steps := 4
	stepH := float32(1.0) / float32(steps)
	stepD := d / float32(steps)

	// Cor um pouco mais escura para os degraus laterais para dar profundidade
	sideColor := [4]uint8{uint8(float32(color[0]) * 0.8), uint8(float32(color[1]) * 0.8), uint8(float32(color[2]) * 0.8), color[3]}

	for i := 0; i < steps; i++ {
		curH := float32(i+1) * stepH
		prevH := float32(i) * stepH
		curZ := z - float32(i)*stepD

		// Topo do degrau - CCW com AO
		buffer.AddFaceAOStandard(
			[3]float32{x, y + curH, curZ}, color,
			[3]float32{x, y + curH, curZ - stepD}, color,
			[3]float32{x + w, y + curH, curZ - stepD}, color,
			[3]float32{x + w, y + curH, curZ}, color,
			[3]float32{0, 1, 0},
		)

		// Frente do degrau (+Z/North) - CCW
		buffer.AddFace(
			[3]float32{x, y + prevH, curZ},
			[3]float32{x + w, y + prevH, curZ},
			[3]float32{x + w, y + curH, curZ},
			[3]float32{x, y + curH, curZ},
			[3]float32{0, 0, 1}, sideColor,
		)

		// Lados do degrau (West/East)
		// West (-X)
		buffer.AddFace(
			[3]float32{x, y + prevH, curZ - stepD},
			[3]float32{x, y + prevH, curZ},
			[3]float32{x, y + curH, curZ},
			[3]float32{x, y + curH, curZ - stepD},
			[3]float32{-1, 0, 0}, sideColor,
		)
		// Leste (+X) - CCW
		buffer.AddFace(
			[3]float32{x + w, y, curZ},
			[3]float32{x + w, y, curZ - stepD},
			[3]float32{x + w, y + curH, curZ - stepD},
			[3]float32{x + w, y + curH, curZ},
			[3]float32{1, 0, 0}, sideColor,
		)
	}

	// Se for StairUpDown ou StairDown, adicionamos um fundo sólido para não ver através do chão
	if shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
		// Piso base
		buffer.AddFace(
			[3]float32{x, y, z},
			[3]float32{x + w, y, z},
			[3]float32{x + w, y, z - d},
			[3]float32{x, y, z - d},
			[3]float32{0, -1, 0}, color,
		)
	}
}

func (m *BlockMesher) addFortification(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Uma fortificação no DF é uma parede com uma fenda horizontal.
	// Vamos fazer:
	// Base: 0.0 -> 0.4
	// Topo: 0.7 -> 1.0
	// Isso deixa uma fenda de 0.3 no meio para ver as flechas (ou anões).

	// Parte de Baixo (0.0 a 0.4)
	m.addCubeGreedy(rl.Vector3{X: x, Y: y, Z: z}, w, 0.4, d, color, false, true, true, true, true, true, buffer, coord, data)

	// Parte de Cima (0.7 a 1.0)
	m.addCubeGreedy(rl.Vector3{X: x, Y: y + 0.7, Z: z}, w, 0.3, d, color, true, false, true, true, true, true, buffer, coord, data)

	// Adicionamos as faces internas da fenda para não ficar "oco"
	// Sola do bloco de cima
	buffer.AddFace(
		[3]float32{x, y + 0.7, z},
		[3]float32{x + w, y + 0.7, z},
		[3]float32{x + w, y + 0.7, z - d},
		[3]float32{x, y + 0.7, z - d},
		[3]float32{0, -1, 0}, color,
	)
	// Teto do bloco de baixo
	buffer.AddFace(
		[3]float32{x, y + 0.4, z},
		[3]float32{x, y + 0.4, z - d},
		[3]float32{x + w, y + 0.4, z - d},
		[3]float32{x + w, y + 0.4, z},
		[3]float32{0, 1, 0}, color,
	)
}

// addTrunkBranch renderiza TRUNK_BRANCH: piso fino (base sólida) + model de tronco pequeno.
// No Armok Vision, TRUNK_BRANCH é tratado como floor + growth layers.
func (m *BlockMesher) addTrunkBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)

	// Piso fino como base do galho-tronco (h=0.15)
	m.addCubeGreedy(pos, 1.0, 0.15, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

	// Model 3D de tronco pequeno sobre o piso
	var rotation float32
	if tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0 {
		dirX := float64(-tile.PositionOnTree.X)
		dirY := float64(tile.PositionOnTree.Y)
		rotation = float32(math.Atan2(dirX, dirY) * (180.0 / math.Pi))
	} else {
		rotation = float32((coord.X*17 + coord.Y*31 + coord.Z*13) % 360)
	}

	modelName := "tree_trunk"
	if tile.MaterialCategory() == dfproto.TilematMushroom {
		modelName = "mushroom"
	}

	scale := float32(1.0) // Modelos do Armok já são 1x1x1
	if tile.TrunkPercent > 0 {
		scale = float32(tile.TrunkPercent) / 100.0
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.15, pos.Z - 0.5},
		Scale:       scale,
		Rotation:    rotation,
		Color:       color,
	})
}

// addBranch renderiza BRANCH: piso fino horizontal + modelo TreeBranches.obj
func (m *BlockMesher) addBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)

	// Piso fino representando o galho horizontal (h=0.08)
	m.addCubeGreedy(pos, 1.0, 0.08, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

	// Adicionar modelo de galho do Armok
	var rotation float32
	if tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0 {
		dirX := float64(-tile.PositionOnTree.X)
		dirY := float64(tile.PositionOnTree.Y)
		rotation = float32(math.Atan2(dirX, dirY) * (180.0 / math.Pi))
	} else {
		rotation = float32((coord.X*17 + coord.Y*31 + coord.Z*13) % 360)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "tree_branches",
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.08, pos.Z - 0.5},
		Scale:       1.0,
		Rotation:    rotation,
		Color:       color,
	})
}

// addTwig renderiza TWIG: piso finíssimo + modelo TreeTwigs.obj
func (m *BlockMesher) addTwig(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)

	// Piso finíssimo como base (h=0.04)
	m.addCubeGreedy(pos, 1.0, 0.04, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

	// Modelo de raminhos com folhas
	var rotation float32
	if tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0 {
		dirX := float64(-tile.PositionOnTree.X)
		dirY := float64(tile.PositionOnTree.Y)
		rotation = float32(math.Atan2(dirX, dirY) * (180.0 / math.Pi))
	} else {
		rotation = float32((coord.X*13 + coord.Y*17 + coord.Z*31) % 360)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "tree_twigs",
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.04, pos.Z - 0.5},
		Scale:       1.0,
		Rotation:    rotation,
		Color:       color,
	})
}

func (m *BlockMesher) calculateAwayFromWallRotation(tile *mapdata.Tile) float32 {
	var vx, vz float32

	// No Raylib (+Z) é Norte, (-Z) é Sul, (-X) é Oeste, (+X) é Leste
	if n := tile.GetNeighbor(util.DirNorth); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vz -= 1.0 // Parede ao Norte -> Empurra para o Sul
	}
	if n := tile.GetNeighbor(util.DirSouth); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vz += 1.0 // Parede ao Sul -> Empurra para o Norte
	}
	if n := tile.GetNeighbor(util.DirWest); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vx += 1.0 // Parede a Oeste -> Empurra para Leste
	}
	if n := tile.GetNeighbor(util.DirEast); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vx -= 1.0 // Parede a Leste -> Empurra para Oeste
	}

	if vx == 0 && vz == 0 {
		return -1 // Sem paredes próximas
	}

	// Converte vetor de empuxo em ângulo
	return float32(math.Atan2(float64(vx), float64(vz))) * (180.0 / math.Pi)
}

func (m *BlockMesher) addShrub(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)

	// 1. Tentar rotação "AwayFromWall" para arbustos adjacentes a paredes
	rotation := m.calculateAwayFromWallRotation(tile)

	// 2. Se não houver parede, usar aleatório determinístico (baseado na coord)
	if rotation == -1 {
		rotation = float32((coord.X*31 + coord.Y*13 + coord.Z*17) % 360)
	} else {
		// Adiciona uma pequena variação aleatória (-15 a +15 graus) para não ficar robótico
		variation := float32((coord.X*7+coord.Y*11)%31) - 15
		rotation += variation
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "shrub",
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.15, pos.Z - 0.5},
		Scale:       0.4,
		Rotation:    rotation,
		Color:       color,
	})
}
