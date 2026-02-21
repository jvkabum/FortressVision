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
	for currentZ := req.FocusZ - 200; currentZ <= req.FocusZ+50; currentZ++ {
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
					m.addCube(pos, w, h, d, currentRunColor, runTile, terrainBuffer)
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

				if tile == nil || tile.Shape() == dfproto.ShapeEmpty || tile.Shape() == dfproto.ShapeNone {
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
						cw1 := m.shouldDrawFace(tile, util.DirWest)
						cw2 := m.shouldDrawFace(runTile, util.DirWest)
						ce1 := m.shouldDrawFace(tile, util.DirEast)
						ce2 := m.shouldDrawFace(runTile, util.DirEast)

						if cw1 != cw2 || ce1 != ce2 {
							canMerge = false
						}
					}
				}

				if !canMerge {
					flushRun()
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

// addCube adiciona um cubo com culling de faces.
func (m *BlockMesher) addCube(pos rl.Vector3, w, h, d float32, color [4]uint8, tile *mapdata.Tile, buffer *MeshBuffer) {
	x, y, z := pos.X, pos.Y, pos.Z

	// Definições das faces (v1, v2, v3, v4)
	// Nota: No nosso sistema, Y é CIMA (Z do DF), Z é SUL (Y do DF).

	// Face Topo (+Y)
	if m.shouldDrawFace(tile, util.DirUp) {
		buffer.AddFace(
			[3]float32{x, y + h, z},
			[3]float32{x, y + h, z - d},
			[3]float32{x + w, y + h, z - d},
			[3]float32{x + w, y + h, z},
			[3]float32{0, 1, 0}, color,
		)
	}

	// Face Baixo (-Y)
	if m.shouldDrawFace(tile, util.DirDown) {
		buffer.AddFace(
			[3]float32{x, y, z},
			[3]float32{x + w, y, z},
			[3]float32{x + w, y, z - d},
			[3]float32{x, y, z - d},
			[3]float32{0, -1, 0}, color,
		)
	}

	// Face Norte (-Z no mundo 3D, norte no DF é Y-)
	// Lembrete: util.DFToWorldPos inverte Y para Z. DF(Y-) -> 3D(Z+) ??
	// No coords.go: Z: float32(-coord.Y) * GameScale
	// Então Norte (Y=10) -> Z=-10. Sul (Y=20) -> Z=-20.
	// Então Norte é "Z maior" (mais perto da origem sonora/visual se considerarmos Y+ para Sul).
	// Vamos simplificar: Usar as direções do DF.

	if m.shouldDrawFace(tile, util.DirNorth) {
		buffer.AddFace(
			[3]float32{x, y, z},
			[3]float32{x, y + h, z},
			[3]float32{x + w, y + h, z},
			[3]float32{x + w, y, z},
			[3]float32{0, 0, 1}, color,
		)
	}

	if m.shouldDrawFace(tile, util.DirSouth) {
		buffer.AddFace(
			[3]float32{x + w, y, z - d},
			[3]float32{x + w, y + h, z - d},
			[3]float32{x, y + h, z - d},
			[3]float32{x, y, z - d},
			[3]float32{0, 0, -1}, color,
		)
	}

	if m.shouldDrawFace(tile, util.DirWest) {
		buffer.AddFace(
			[3]float32{x, y, z - d},
			[3]float32{x, y + h, z - d},
			[3]float32{x, y + h, z},
			[3]float32{x, y, z},
			[3]float32{-1, 0, 0}, color,
		)
	}

	if m.shouldDrawFace(tile, util.DirEast) {
		buffer.AddFace(
			[3]float32{x + w, y, z},
			[3]float32{x + w, y + h, z},
			[3]float32{x + w, y + h, z - d},
			[3]float32{x + w, y, z - d},
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
	if tile.Shape() == dfproto.ShapeWall && neighborShape == dfproto.ShapeWall {
		return false
	}

	// Caso especial: Piso em cima de parede sólida
	if dir == util.DirDown && tile.Shape() == dfproto.ShapeFloor && neighborShape == dfproto.ShapeWall {
		return false
	}

	return true
}
