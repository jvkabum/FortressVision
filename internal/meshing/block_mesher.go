package meshing

import (
	"FortressVision/internal/mapdata"
	"FortressVision/internal/util"
	"FortressVision/pkg/dfproto"
	"log"
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
		Origin: req.Origin,
		MTime:  req.MTime,
	}

	terrainBuffer := GetMeshBuffer()
	liquidBuffer := GetMeshBuffer()

	// Resetamos forçadamente na alocação caso o slice não esteja limpo
	terrainBuffer.Geometry.Vertices = terrainBuffer.Geometry.Vertices[:0]
	terrainBuffer.Geometry.Normals = terrainBuffer.Geometry.Normals[:0]
	terrainBuffer.Geometry.Colors = terrainBuffer.Geometry.Colors[:0]

	liquidBuffer.Geometry.Vertices = liquidBuffer.Geometry.Vertices[:0]
	liquidBuffer.Geometry.Normals = liquidBuffer.Geometry.Normals[:0]
	liquidBuffer.Geometry.Colors = liquidBuffer.Geometry.Colors[:0]

	// Algoritmo Greedy Meshing Vertical (1D Y-Axis)
	// Para cada fatia (Z e X constantes), varremos Y procurando paredes e chãos contínuos
	// para fundir as faces e economizar vértices.
	runGreedyMesherX(req, terrainBuffer, liquidBuffer, m)

	if len(terrainBuffer.Geometry.Vertices) > 0 || len(liquidBuffer.Geometry.Vertices) > 0 {
		// Log reduzido para evitar flood
	}

	res.Terreno = terrainBuffer.Geometry.Clone()
	res.Liquidos = liquidBuffer.Geometry.Clone()
	return res
}

func runGreedyMesherX(req Request, terrainBuffer *MeshBuffer, liquidBuffer *MeshBuffer, m *BlockMesher) {
	// Chunks do DFHack são 16x16x1. O origin.Z é a camada correta.
	currentZ := req.Origin.Z
	{
		for xx := int32(0); xx < 16; xx++ {
			var runStartY int32 = -1
			var runLength int32 = 0
			var currentRunShape dfproto.TiletypeShape
			var currentRunColor [4]uint8
			var runTile *mapdata.Tile

			flushRun := func() {
				if runLength > 0 && runTile != nil {
					// Quando terminamos uma fita ininterrupta, geramos a geometria.
					pos := util.DFToWorldPos(util.DFCoord{X: req.Origin.X + xx, Y: req.Origin.Y + runStartY, Z: int32(currentZ)})

					w := float32(1.0)
					d := float32(runLength) // No raylib Z avança quando Y do DF avança (negativamente ou não, tratado no addCube)
					h := float32(1.0)
					if currentRunShape == dfproto.ShapeFloor {
						h = 0.1
					}

					// Construir flags de face baseados na fita
					// A fita anda no eixo Y do DF (Norte->Sul).
					// Logo, a face Norte da FITA é a face Norte do primeiro tile (runTile).
					// A face Sul da FITA é a face Sul do último tile (o tile anterior à quebra).
					// As faces laterais, topo e baixo são idênticas em todos os tiles da fita!
					endTileY := runStartY + runLength - 1
					endWorldCoord := util.NewDFCoord(req.Origin.X+xx, req.Origin.Y+endTileY, int32(currentZ))
					endTile := req.Data.GetTile(endWorldCoord)

					drawUp := m.shouldDrawFace(runTile, util.DirUp)
					drawDown := m.shouldDrawFace(runTile, util.DirDown)
					drawWest := m.shouldDrawFace(runTile, util.DirWest)
					drawEast := m.shouldDrawFace(runTile, util.DirEast)
					drawNorth := m.shouldDrawFace(runTile, util.DirNorth)

					var drawSouth bool
					if endTile != nil {
						drawSouth = m.shouldDrawFace(endTile, util.DirSouth)
					} else {
						drawSouth = true // fallback se nao achar no mapa carregado (borda)
					}

					m.addCubeGreedy(pos, w, h, d, currentRunColor, drawUp, drawDown, drawNorth, drawSouth, drawWest, drawEast, terrainBuffer, util.DFCoord{X: req.Origin.X + xx, Y: req.Origin.Y + runStartY, Z: currentZ}, req.Data)
				}
				runStartY = -1
				runLength = 0
				runTile = nil
			}

			for yy := int32(0); yy < 16; yy++ {
				baseX := req.Origin.X + xx
				baseY := req.Origin.Y + yy
				worldCoord := util.NewDFCoord(baseX, baseY, int32(currentZ))
				tile := req.Data.GetTile(worldCoord)

				if tile == nil || tile.Hidden || tile.Shape() == dfproto.ShapeEmpty || tile.Shape() == dfproto.ShapeNone {
					flushRun()
					continue
				}

				GenerateLiquidGeometry(tile, liquidBuffer)

				shape := tile.Shape()
				rlColor := m.MatStore.GetTileColor(tile)
				color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

				// Verifica quebra de fita (Cor ou formato diferente, ou necessidades de culling lateral diferentes)
				// Na nossa engine simplificada, quebramos fita se a face Oeste ou Leste mudar de visibilidade.
				canMerge := true
				if runLength > 0 {
					if shape != currentRunShape || color != currentRunColor {
						canMerge = false
					} else {
						// Para fundir blocos na mesma fita, TODAS as faces (Top, Bottom, West, East)
						// visíveis devem bater, exceto o eixo do movimento (North/South).
						// Se a fita anda em Y (Norte para Sul), o Teto de um tem q ser = o Teto do outro.
						if m.shouldDrawFace(tile, util.DirWest) != m.shouldDrawFace(runTile, util.DirWest) ||
							m.shouldDrawFace(tile, util.DirEast) != m.shouldDrawFace(runTile, util.DirEast) ||
							m.shouldDrawFace(tile, util.DirUp) != m.shouldDrawFace(runTile, util.DirUp) ||
							m.shouldDrawFace(tile, util.DirDown) != m.shouldDrawFace(runTile, util.DirDown) {
							canMerge = false
						}
					}
				}

				if !canMerge {
					flushRun()
				}

				if shape == dfproto.ShapeRamp {
					flushRun() // Rampas não são mescladas no greedy mesher por enquanto
					m.addRamp(worldCoord, color, terrainBuffer, req.Data)
					continue
				}

				if shape == dfproto.ShapeStairUp || shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
					flushRun()
					m.addStairs(worldCoord, shape, color, terrainBuffer, req.Data)
					continue
				}

				if shape == dfproto.ShapeTreeShape || shape == dfproto.ShapeTrunkBranch {
					flushRun()
					m.addTreeTrunk(worldCoord, color, terrainBuffer, req.Data)
					continue
				}

				if shape == dfproto.ShapeBranch || shape == dfproto.ShapeTwig {
					flushRun()
					m.addTreeLeaves(worldCoord, color, terrainBuffer, req.Data)
					continue
				}

				if shape == dfproto.ShapeSapling || shape == dfproto.ShapeShrub {
					flushRun()
					m.addShrub(worldCoord, color, terrainBuffer, req.Data)
					continue
				}

				if runStartY == -1 {
					runStartY = yy
					currentRunShape = shape
					currentRunColor = color
					runTile = tile
				}
				runLength++
			}
			flushRun() // Descarrega a ponta da fita se acabou o chunk no Y=15
		}
	}
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

		buffer.AddFaceAOStandard(
			[3]float32{x, y + h, z}, applyAO(color, aoNW), // NW
			[3]float32{x + w, y + h, z}, applyAO(color, aoNE), // NE
			[3]float32{x + w, y + h, z - d}, applyAO(color, aoSE), // SE
			[3]float32{x, y + h, z - d}, applyAO(color, aoSW), // SW
			[3]float32{0, 1, 0},
		)
	}

	// Face Baixo (-Y)
	if drawDown {
		buffer.AddFace(
			[3]float32{x, y, z},         // NW
			[3]float32{x, y, z - d},     // SW
			[3]float32{x + w, y, z - d}, // SE
			[3]float32{x + w, y, z},     // NE
			[3]float32{0, -1, 0}, color,
		)
	}

	// Face Norte (+Z)
	if drawNorth {
		buffer.AddFace(
			[3]float32{x, y, z},         // Bottom-NW
			[3]float32{x + w, y, z},     // Bottom-NE
			[3]float32{x + w, y + h, z}, // Top-NE
			[3]float32{x, y + h, z},     // Top-NW
			[3]float32{0, 0, 1}, color,
		)
	}

	// Face Sul (-Z)
	if drawSouth {
		buffer.AddFace(
			[3]float32{x + w, y, z - d},     // Bottom-SE
			[3]float32{x, y, z - d},         // Bottom-SW
			[3]float32{x, y + h, z - d},     // Top-SW
			[3]float32{x + w, y + h, z - d}, // Top-SE
			[3]float32{0, 0, -1}, color,
		)
	}

	// Face Oeste (-X)
	if drawWest {
		buffer.AddFace(
			[3]float32{x, y, z - d},     // Bottom-SW
			[3]float32{x, y, z},         // Bottom-NW
			[3]float32{x, y + h, z},     // Top-NW
			[3]float32{x, y + h, z - d}, // Top-SW
			[3]float32{-1, 0, 0}, color,
		)
	}

	// Face Leste (+X)
	if drawEast {
		buffer.AddFace(
			[3]float32{x + w, y, z},         // Bottom-NE
			[3]float32{x + w, y, z - d},     // Bottom-SE
			[3]float32{x + w, y + h, z - d}, // Top-SE
			[3]float32{x + w, y + h, z},     // Top-NE
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

	// Se o vizinho for vazio/transparente, desenha
	neighborShape := neighbor.Shape()
	if neighborShape == dfproto.ShapeEmpty || neighborShape == dfproto.ShapeNone {
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

func (m *BlockMesher) addRamp(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Direções que têm conexão
	north := m.isSolid(coord, util.DirNorth, data)
	south := m.isSolid(coord, util.DirSouth, data)
	east := m.isSolid(coord, util.DirEast, data)
	west := m.isSolid(coord, util.DirWest, data)

	// Geometria básica: Piso
	buffer.AddFace(
		[3]float32{x, y, z},
		[3]float32{x + w, y, z},
		[3]float32{x + w, y, z - d},
		[3]float32{x, y, z - d},
		[3]float32{0, -1, 0}, color,
	)

	// Alturas dos cantos (0 = baixo, 1 = alto)
	// DF Coord: North is Y-1, South is Y+1
	// World Coord: North is +Z, South is -Z (based on coords.go: Z: float32(-coord.Y) * GameScale)
	// Wait, coords.go says:
	// DirNorth: {X: 0, Y: -1, Z: 0} -> World Pos Z: -(Y-1) = -Y + 1 (moved North)
	// So North is +Z, South is -Z.
	hNW, hNE, hSE, hSW := float32(0.0), float32(0.0), float32(0.0), float32(0.0)

	if north {
		hNW, hNE = 1, 1
	}
	if south {
		hSW, hSE = 1, 1
	}
	if east {
		hNE, hSE = 1, 1
	}
	if west {
		hNW, hSW = 1, 1
	}

	// Se for uma rampa isolada (sem vizinhos solidos), vira um bloco baixo (piso)
	if !north && !south && !east && !west {
		hNW, hNE, hSE, hSW = 0.1, 0.1, 0.1, 0.1
	}

	// Vértices do topo
	vNW := [3]float32{x, y + hNW, z}
	vNE := [3]float32{x + w, y + hNE, z}
	vSE := [3]float32{x + w, y + hSE, z - d}
	vSW := [3]float32{x, y + hSW, z - d}

	// Vértices da base
	bNW := [3]float32{x, y, z}
	bNE := [3]float32{x + w, y, z}
	bSE := [3]float32{x + w, y, z - d}
	bSW := [3]float32{x, y, z - d}

	// Face de cima (Rampa) - CCW
	buffer.AddFace(vNW, vSW, vSE, vNE, [3]float32{0, 1, 0}, color)

	// Proteger as laterais se houver desnível - CCW
	if hNW > 0 || hNE > 0 { // Face Norte (+Z)
		buffer.AddFace(bNW, bNE, vNE, vNW, [3]float32{0, 0, 1}, color)
	}
	if hSW > 0 || hSE > 0 { // Face Sul (-Z)
		buffer.AddFace(bSE, bSW, vSW, vSE, [3]float32{0, 0, -1}, color)
	}
	if hNW > 0 || hSW > 0 { // Face Oeste (-X)
		buffer.AddFace(bSW, bNW, vNW, vSW, [3]float32{-1, 0, 0}, color)
	}
	if hNE > 0 || hSE > 0 { // Face Leste (+X)
		buffer.AddFace(bNE, bSE, vSE, vNE, [3]float32{1, 0, 0}, color)
	}
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

func (m *BlockMesher) addTreeTrunk(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z

	// Cor de madeira mais escura para o tronco
	trunkColor := [4]uint8{uint8(float32(color[0]) * 0.8), uint8(float32(color[1]) * 0.8), uint8(float32(color[2]) * 0.8), 255}

	// Simplificamos o tronco como um paralelepípedo mais fino (0.4x0.4)
	// para não parecer apenas um bloco sólido
	o := float32(0.3) // Offset para centralizar
	tw, td := float32(0.4), float32(0.4)

	// Face Norte (+Z)
	buffer.AddFaceAOStandard([3]float32{x + o, y, z - o}, trunkColor, [3]float32{x + o + tw, y, z - o}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o}, trunkColor, [3]float32{x + o, y + 1.0, z - o}, trunkColor, [3]float32{0, 0, 1})
	// Face Sul (-Z)
	buffer.AddFaceAOStandard([3]float32{x + o, y, z - o - td}, trunkColor, [3]float32{x + o, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o + tw, y, z - o - td}, trunkColor, [3]float32{0, 0, -1})
	// Face Oeste (-X)
	buffer.AddFaceAOStandard([3]float32{x + o, y, z - o}, trunkColor, [3]float32{x + o, y + 1.0, z - o}, trunkColor, [3]float32{x + o, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o, y, z - o - td}, trunkColor, [3]float32{-1, 0, 0})
	// Face Leste (+X)
	buffer.AddFaceAOStandard([3]float32{x + o + tw, y, z - o}, trunkColor, [3]float32{x + o + tw, y, z - o - td}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o}, trunkColor, [3]float32{1, 0, 0})

	// Topo e Baixo se necessário (geralmente cobertos por outros troncos ou folhas)
	buffer.AddFaceAOStandard([3]float32{x + o, y + 1.0, z - o}, trunkColor, [3]float32{x + o, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o - td}, trunkColor, [3]float32{x + o + tw, y + 1.0, z - o}, trunkColor, [3]float32{0, 1, 0})
}

func (m *BlockMesher) addTreeLeaves(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z

	// Para folhas, usamos um bloco levemente menor (0.8) e centralizado para dar aspecto de "nuget" de folhas
	o := float32(0.1)
	s := float32(0.8)

	// Cor esverdeada para folhas (DF pode mandar cores variadas, vamos garantir o alpha)
	leafColor := color
	leafColor[3] = 255

	// 6 Faces CCW
	buffer.AddFaceAOStandard([3]float32{x + o, y + o, z - o}, leafColor, [3]float32{x + o + s, y + o, z - o}, leafColor, [3]float32{x + o + s, y + o + s, z - o}, leafColor, [3]float32{x + o, y + o + s, z - o}, leafColor, [3]float32{0, 0, 1})
	buffer.AddFaceAOStandard([3]float32{x + o, y + o, z - o - s}, leafColor, [3]float32{x + o, y + o + s, z - o - s}, leafColor, [3]float32{x + o + s, y + o + s, z - o - s}, leafColor, [3]float32{x + o + s, y + o, z - o - s}, leafColor, [3]float32{0, 0, -1})
	buffer.AddFaceAOStandard([3]float32{x + o, y + o, z - o}, leafColor, [3]float32{x + o, y + o + s, z - o}, leafColor, [3]float32{x + o, y + o + s, z - o - s}, leafColor, [3]float32{x + o, y + o, z - o - s}, leafColor, [3]float32{-1, 0, 0})
	buffer.AddFaceAOStandard([3]float32{x + o + s, y + o, z - o}, leafColor, [3]float32{x + o + s, y + o, z - o - s}, leafColor, [3]float32{x + o + s, y + o + s, z - o - s}, leafColor, [3]float32{x + o + s, y + o + s, z - o}, leafColor, [3]float32{1, 0, 0})
	buffer.AddFaceAOStandard([3]float32{x + o, y + o + s, z - o}, leafColor, [3]float32{x + o, y + o + s, z - o - s}, leafColor, [3]float32{x + o + s, y + o + s, z - o - s}, leafColor, [3]float32{x + o + s, y + o + s, z - o}, leafColor, [3]float32{0, 1, 0})
	buffer.AddFaceAOStandard([3]float32{x + o, y + o, z - o}, leafColor, [3]float32{x + o + s, y + o, z - o}, leafColor, [3]float32{x + o + s, y + o, z - o - s}, leafColor, [3]float32{x + o, y + o, z - o - s}, leafColor, [3]float32{0, -1, 0})
}

func (m *BlockMesher) addShrub(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z

	// Arbustos são pequenos "X" (cross-quads) como em jogos retro
	c := color
	c[3] = 255

	// Diagonal 1
	buffer.AddFace([3]float32{x, y, z}, [3]float32{x + 1, y, z - 1}, [3]float32{x + 1, y + 0.7, z - 1}, [3]float32{x, y + 0.7, z}, [3]float32{0, 1, 0}, c)
	// Diagonal 2
	buffer.AddFace([3]float32{x + 1, y, z}, [3]float32{x, y, z - 1}, [3]float32{x, y + 0.7, z - 1}, [3]float32{x + 1, y + 0.7, z}, [3]float32{0, 1, 0}, c)
}
