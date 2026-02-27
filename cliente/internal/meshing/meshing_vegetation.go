package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"math"
	"strings"
)

// addTrunk renderiza TRUNK ou TRUNK_BRANCH: base sólida mínima + modelo de tronco.
func (m *BlockMesher) addTrunk(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())

	// Adicionamos um piso finíssimo (h=0.05) para manter oclusão e suporte visual no chão
	buf := getBuffer(texName)
	m.addCubeGreedy(pos, 1.0, 0.05, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

	// Model 3D de tronco principal
	modelName := "tree_trunk" // TreeTrunkPillar.obj - Mais leve e sólido
	if tile.MaterialCategory() == dfproto.TilematMushroom {
		modelName = "mushroom"
	}

	// Rotação baseada na posição na árvore (conforme Armok's TreeFlat/TreeRound)
	var rotation float32
	if tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0 {
		dirX := float64(-tile.PositionOnTree.X)
		dirY := float64(tile.PositionOnTree.Y)
		rotation = float32(math.Atan2(dirX, dirY) * (180.0 / math.Pi))
	} else {
		rotation = 0
	}

	scale := float32(0.33)
	if tile.TrunkPercent > 0 {
		scale = (float32(tile.TrunkPercent) / 100.0) * 0.33
	}

	// Diferenciação de cor: Troncos de madeira tendem a ser um pouco mais escuros que o solo
	trunkColor := color
	if tile.MaterialCategory() == dfproto.TilematTreeMaterial || tile.MaterialCategory() == dfproto.TilematMushroom {
		trunkColor[0] = uint8(float32(color[0]) * 0.9)
		trunkColor[1] = uint8(float32(color[1]) * 0.85)
		trunkColor[2] = uint8(float32(color[2]) * 0.8)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y - 0.2, pos.Z - 0.5},
		Scale:       scale,
		Rotation:    rotation,
		Color:       trunkColor,
	})
}

// addBranch renderiza BRANCH: piso fino horizontal + modelo TreeBranches.obj
func (m *BlockMesher) addBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)

	m.addCubeGreedy(pos, 1.0, 0.08, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

	connectivity := ""
	if ttype, ok := data.Tiletypes[tile.TileType]; ok {
		connectivity = ttype.Dir
	}

	modelName := "tree_branches"
	rotation := float32(0)

	if connectivity != "" && connectivity != "--------" {
		n, s, e, w := connectivity[0] == 'N', connectivity[1] == 'S', connectivity[2] == 'E', connectivity[3] == 'W'
		suffix := ""
		if n {
			suffix += "N"
		}
		if s {
			suffix += "S"
		}
		if e {
			suffix += "E"
		}
		if w {
			suffix += "W"
		}

		switch suffix {
		case "NS", "EW", "NE", "NW", "SE", "SW", "NSE", "NSW", "NEW", "SEW", "NSEW":
			modelName = "tree_branch_" + strings.ToLower(suffix)
		default:
			modelName = "tree_branches"
			rotation = float32((coord.X*17+coord.Y*31)%4) * 90.0
		}
	} else {
		rotation = float32((coord.X*17 + coord.Y*31) % 360)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.08, pos.Z - 0.5},
		Scale:       0.33,
		Rotation:    rotation,
		Color:       color,
	})
}

func (m *BlockMesher) addTwig(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)

	m.addCubeGreedy(pos, 1.0, 0.04, 1.0, color,
		true, m.shouldDrawFace(tile, util.DirDown),
		m.shouldDrawFace(tile, util.DirNorth), m.shouldDrawFace(tile, util.DirSouth),
		m.shouldDrawFace(tile, util.DirWest), m.shouldDrawFace(tile, util.DirEast),
		buf, coord, data)

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
		Scale:       0.33,
		Rotation:    rotation,
		Color:       color,
	})
}

func (m *BlockMesher) addShrub(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	rotation := m.calculateAwayFromWallRotation(tile)

	if rotation == -1 {
		rotation = float32((coord.X*31 + coord.Y*13 + coord.Z*17) % 360)
	} else {
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

func (m *BlockMesher) addGrass(coord util.DFCoord, tile *mapdata.Tile, res *Result) {
	pos := util.DFToWorldPos(coord)
	rlColor := m.MatStore.GetTileColor(tile)
	color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

	count := 1
	if tile.GrassPercent > 50 {
		count = 2
	}
	if tile.GrassPercent > 85 {
		count = 3
	}

	for i := 0; i < count; i++ {
		seed := int32(coord.X*7 + coord.Y*13 + int32(i))
		offX := float32(seed%10) / 10.0
		offZ := float32((seed/10)%10) / 10.0
		rotation := float32(seed % 360)

		res.ModelInstances = append(res.ModelInstances, ModelInstance{
			ModelName:   "shrub",
			TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
			Position:    [3]float32{pos.X + offX, pos.Y + 0.1, pos.Z - offZ},
			Scale:       0.2 + (float32(seed%5) / 10.0),
			Rotation:    rotation,
			Color:       color,
		})
	}
}
