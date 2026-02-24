package mapdata

import (
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

// Tile representa um único tile do mapa do Dwarf Fortress.
// Baseado na classe Tile aninhada em MapDataStore.cs do Armok Vision.
type Tile struct {
	container *MapDataStore
	Position  util.DFCoord

	TileType int32
	Material dfproto.MatPair

	BaseMaterial     dfproto.MatPair
	LayerMaterial    dfproto.MatPair
	VeinMaterial     dfproto.MatPair
	ConstructionItem dfproto.MatPair

	WaterLevel int32
	MagmaLevel int32

	// FlowVector armazena a direção do fluxo do líquido (-1, 0, 1 para X e Y)
	FlowVector util.DFCoord

	RampType int32
	Hidden   bool

	TrunkPercent   uint8
	PositionOnTree util.DFCoord

	DigDesignation dfproto.TileDigDesignation

	// GrassPercent armazena a quantidade de grama no tile (0-100)
	GrassPercent int32

	// Spatters armazena informações de sujeira/sangue/etc (opcional por enquanto)
	// Spatters []dfproto.Spatter
}

// NewTile cria um novo tile inicializado.
func NewTile(container *MapDataStore, pos util.DFCoord) *Tile {
	return &Tile{
		container: container,
		Position:  pos,
	}
}

// CopyFrom copia os dados de outro tile.
func (t *Tile) CopyFrom(orig *Tile) {
	t.TileType = orig.TileType
	t.Material = orig.Material
	t.BaseMaterial = orig.BaseMaterial
	t.LayerMaterial = orig.LayerMaterial
	t.VeinMaterial = orig.VeinMaterial
	t.ConstructionItem = orig.ConstructionItem
	t.WaterLevel = orig.WaterLevel
	t.MagmaLevel = orig.MagmaLevel
	t.FlowVector = orig.FlowVector
	t.RampType = orig.RampType
	t.Hidden = orig.Hidden
	t.TrunkPercent = orig.TrunkPercent
	t.PositionOnTree = orig.PositionOnTree
	t.DigDesignation = orig.DigDesignation
	t.GrassPercent = orig.GrassPercent
}

// Propriedades auxiliares baseadas nos tipos (necessita que o container tenha acesso aos raws)

func (t *Tile) Shape() dfproto.TiletypeShape {
	if t.container == nil || t.container.Tiletypes == nil {
		return dfproto.ShapeNone
	}
	tt, ok := t.container.Tiletypes[t.TileType]
	if !ok {
		return dfproto.ShapeNone
	}
	return tt.Shape
}

func (t *Tile) MaterialCategory() dfproto.TiletypeMaterial {
	if t.container == nil || t.container.Tiletypes == nil {
		return dfproto.TilematNone
	}
	tt, ok := t.container.Tiletypes[t.TileType]
	if !ok {
		return dfproto.TilematNone
	}
	return tt.Material
}

func (t *Tile) IsWall() bool {
	shape := t.Shape()
	switch shape {
	case dfproto.ShapeWall, dfproto.ShapeTreeShape:
		return true
	default:
		return false
	}
}

func (t *Tile) IsFloor() bool {
	shape := t.Shape()
	switch shape {
	case dfproto.ShapeRamp, dfproto.ShapeFloor, dfproto.ShapeBoulder, dfproto.ShapePebbles,
		dfproto.ShapeSapling, dfproto.ShapeShrub, dfproto.ShapeBranch, dfproto.ShapeTrunkBranch:
		return true
	default:
		return false
	}
}

// Vizinhos (Helpers)

// GetStore retorna o MapDataStore que contém este tile.
func (t *Tile) GetStore() *MapDataStore {
	return t.container
}

// GetNeighbor retorna o tile vizinho na direção especificada.
func (t *Tile) GetNeighbor(dir util.Directions) *Tile {
	if t.container == nil {
		return nil
	}
	return t.container.GetTile(t.Position.Add(util.DirOffsets[dir]))
}

func (t *Tile) North() *Tile {
	return t.GetNeighbor(util.DirNorth)
}
func (t *Tile) South() *Tile {
	return t.GetNeighbor(util.DirSouth)
}
func (t *Tile) East() *Tile {
	return t.GetNeighbor(util.DirEast)
}
func (t *Tile) West() *Tile {
	return t.GetNeighbor(util.DirWest)
}
func (t *Tile) Up() *Tile {
	return t.GetNeighbor(util.DirUp)
}
func (t *Tile) Down() *Tile {
	return t.GetNeighbor(util.DirDown)
}

// SetStore vincula o tile a um container MapDataStore.
func (t *Tile) SetStore(s *MapDataStore) {
	t.container = s
}

// Tabela de lookup para rampas (Ramp Lookup Table - BLUT)
// Copiada diretamente do MapDataStore.cs:936
var rampblut = []byte{
	1, 2, 8, 2, 4, 12, 4, 12, 9, 2, 21, 2, 4, 12, 4, 12,
	5, 16, 5, 16, 13, 13, 13, 12, 5, 16, 5, 16, 13, 13, 13, 16,
	7, 2, 14, 2, 4, 12, 4, 12, 20, 26, 25, 26, 4, 12, 4, 12,
	5, 16, 5, 16, 13, 16, 13, 16, 5, 16, 5, 16, 13, 16, 13, 16,
	3, 10, 3, 10, 17, 12, 17, 12, 3, 10, 26, 10, 17, 17, 17, 17,
	11, 10, 11, 16, 11, 26, 17, 12, 11, 16, 11, 16, 13, 13, 17, 16,
	3, 10, 3, 10, 17, 17, 17, 17, 3, 10, 26, 10, 17, 17, 17, 17,
	11, 11, 11, 16, 11, 11, 17, 14, 11, 16, 11, 16, 17, 17, 17, 13,
	6, 2, 19, 2, 4, 12, 4, 12, 15, 2, 24, 2, 4, 12, 4, 12,
	5, 16, 26, 16, 13, 16, 13, 16, 5, 16, 26, 16, 13, 16, 13, 16,
	18, 2, 22, 2, 26, 12, 26, 12, 23, 26, 26, 26, 26, 12, 26, 12,
	5, 16, 26, 16, 13, 16, 13, 16, 5, 16, 26, 16, 13, 16, 13, 16,
	3, 10, 3, 10, 17, 10, 17, 17, 3, 10, 26, 10, 17, 17, 17, 17,
	11, 10, 11, 16, 17, 10, 17, 17, 11, 16, 11, 16, 17, 15, 17, 12,
	3, 10, 3, 10, 17, 17, 17, 17, 3, 10, 26, 10, 17, 17, 17, 17,
	11, 16, 11, 16, 17, 16, 17, 10, 11, 16, 11, 16, 17, 11, 17, 26,
}

// CalculateRampType calcula o tipo de rampa baseado nos vizinhos.
func (t *Tile) CalculateRampType() {
	if t.Shape() != dfproto.ShapeRamp {
		t.RampType = 0
		return
	}

	lookup := 0

	checkWallFloor := func(dir util.Directions) bool {
		neighbor := t.container.GetTile(t.Position.Add(util.DirOffsets[dir]))
		if neighbor == nil {
			return false
		}
		if neighbor.IsWall() {
			up := neighbor.Up()
			if up != nil && up.IsFloor() {
				return true
			}
		}
		return false
	}

	// Fase 1: Paredes com andares em cima (mais específico)
	if checkWallFloor(util.DirNorth) {
		lookup ^= 1
	}
	if checkNeighborWallFloor(t, util.DirNorth, util.DirEast) {
		lookup ^= 2
	}
	if checkWallFloor(util.DirEast) {
		lookup ^= 4
	}
	if checkNeighborWallFloor(t, util.DirSouth, util.DirEast) {
		lookup ^= 8
	}
	if checkWallFloor(util.DirSouth) {
		lookup ^= 16
	}
	if checkNeighborWallFloor(t, util.DirSouth, util.DirWest) {
		lookup ^= 32
	}
	if checkWallFloor(util.DirWest) {
		lookup ^= 64
	}
	if checkNeighborWallFloor(t, util.DirNorth, util.DirWest) {
		lookup ^= 128
	}

	if lookup > 0 {
		t.RampType = int32(rampblut[lookup])
		return
	}

	// Fase 2: Apenas paredes (fallback)
	checkWall := func(dir util.Directions) bool {
		neighbor := t.container.GetTile(t.Position.Add(util.DirOffsets[dir]))
		return neighbor != nil && neighbor.IsWall()
	}

	if checkWall(util.DirNorth) {
		lookup ^= 1
	}
	// Diagonais precisam ser calculadas na mão pois util.DirOffsets só tem as 8 direções básicas
	if checkDiagWall(t, util.DirNorth, util.DirEast) {
		lookup ^= 2
	}
	if checkWall(util.DirEast) {
		lookup ^= 4
	}
	if checkDiagWall(t, util.DirSouth, util.DirEast) {
		lookup ^= 8
	}
	if checkWall(util.DirSouth) {
		lookup ^= 16
	}
	if checkDiagWall(t, util.DirSouth, util.DirWest) {
		lookup ^= 32
	}
	if checkWall(util.DirWest) {
		lookup ^= 64
	}
	if checkDiagWall(t, util.DirNorth, util.DirWest) {
		lookup ^= 128
	}

	t.RampType = int32(rampblut[lookup])
}

// Helpers para diagonais e condições complexas

func checkNeighborWallFloor(t *Tile, d1, d2 util.Directions) bool {
	pos := t.Position.Add(util.DirOffsets[d1]).Add(util.DirOffsets[d2])
	neighbor := t.container.GetTile(pos)
	if neighbor != nil && neighbor.IsWall() {
		up := neighbor.Up()
		return up != nil && up.IsFloor()
	}
	return false
}

func checkDiagWall(t *Tile, d1, d2 util.Directions) bool {
	pos := t.Position.Add(util.DirOffsets[d1]).Add(util.DirOffsets[d2])
	neighbor := t.container.GetTile(pos)
	return neighbor != nil && neighbor.IsWall()
}
