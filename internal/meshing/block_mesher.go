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

	terrainBuffer := &MeshBuffer{}
	liquidBuffer := &MeshBuffer{}

	visibleCount := 0
	// Algoritmo de Raycasting Vertical (Coluna)
	// Para cada posição X,Y na fatia (chunk), iteramos de cima para baixo (do FocusZ)
	// desenhando apenas o que é visível a partir do céu/câmera.

	for xx := int32(0); xx < 16; xx++ {
		for yy := int32(0); yy < 16; yy++ {
			// 1. Verificação de Escopo Vertical (Aumentado para Unlimited Vision)
			// Permitimos uma margem maior para ver o mundo inteiro.
			// Coordenada base X,Y dentro do chunk
			baseX := req.Origin.X + xx
			baseY := req.Origin.Y + yy

			focusZ := int32(req.FocusZ)
			currentZ := req.Origin.Z

			if currentZ > focusZ+50 || currentZ < focusZ-200 {
				continue
			}

			// 2. Verificação de Oclusão DESATIVADA para Unlimited Vision
			// No modo Unlimited Vision, processamos cada nível independentemente da obstrução acima.
			visible := true
			/*
				if currentZ < focusZ {
					for z := focusZ; z > currentZ; z-- {
						checkCoord := util.NewDFCoord(baseX, baseY, z)
						checkTile := req.Data.GetTile(checkCoord)
						if checkTile == nil {
							continue
						}
						shape := checkTile.Shape()
						if shape == dfproto.ShapeFloor || shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification {
							visible = false
							break
						}
					}
				}
			*/

			if !visible {
				continue
			}

			// Renderiza o tile atual
			worldCoord := util.NewDFCoord(baseX, baseY, currentZ)
			tile := req.Data.GetTile(worldCoord)
			if tile == nil {
				continue
			}

			visibleCount++

			// 1. Geometria Sólida (Terreno)
			shape := tile.Shape()
			if shape != dfproto.ShapeEmpty && shape != dfproto.ShapeNone {
				m.generateTileGeometry(tile, terrainBuffer)
			}

			// 2. Geometria de Líquidos
			GenerateLiquidGeometry(tile, liquidBuffer)
		}
	}

	if len(terrainBuffer.Geometry.Vertices) > 0 || len(liquidBuffer.Geometry.Vertices) > 0 {
		// Log reduzido para evitar flood apenas com contagem
		// log.Printf("[Mesher] %s: %d tiles visíveis.", req.Origin.String(), visibleCount)
	}

	res.Terreno = terrainBuffer.Geometry.Clone()
	res.Liquidos = liquidBuffer.Geometry.Clone()
	return res
}

func (m *BlockMesher) generateTileGeometry(tile *mapdata.Tile, buffer *MeshBuffer) {
	shape := tile.Shape()
	pos := util.DFToWorldPos(tile.Position)

	// Cor baseada no material
	rlColor := m.MatStore.GetTileColor(tile)
	color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

	switch shape {
	case dfproto.ShapeWall:
		m.addCube(pos, 1.0, 1.0, 1.0, color, tile, buffer)
	case dfproto.ShapeFloor:
		m.addCube(pos, 1.0, 0.1, 1.0, color, tile, buffer) // Chão fino por enquanto
	}
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
