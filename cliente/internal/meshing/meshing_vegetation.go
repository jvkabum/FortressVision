package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"math"
)

// addTrunk renderiza TRUNK ou TRUNK_BRANCH: base sólida mínima + modelo de tronco.
func (m *BlockMesher) addTrunk(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)

	// Model 3D de tronco principal
	modelName := "tree_trunk" // TreeTrunkPillar.obj - Sólido
	if tile.MaterialCategory() == dfproto.TilematMushroom {
		modelName = "mushroom"
	}

	// Rotação baseada na posição na árvore
	var rotation float32
	if tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0 {
		dirX := float64(-tile.PositionOnTree.X)
		dirY := float64(tile.PositionOnTree.Y)
		rotation = float32(math.Atan2(dirX, dirY) * (180.0 / math.Pi))
	} else {
		rotation = 0
	}

	scale := float32(1.0)
	if tile.TrunkPercent > 0 {
		scale = (float32(tile.TrunkPercent) / 100.0)
	}

	// Diferenciação de cor: Troncos tendem a ser um pouco mais escuros
	trunkColor := color
	if tile.MaterialCategory() == dfproto.TilematTreeMaterial || tile.MaterialCategory() == dfproto.TilematMushroom {
		trunkColor[0] = uint8(float32(color[0]) * 0.9)
		trunkColor[1] = uint8(float32(color[1]) * 0.85)
		trunkColor[2] = uint8(float32(color[2]) * 0.8)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y, pos.Z - 0.5},
		Scale:       scale,
		Rotation:    rotation,
		Color:       trunkColor,
	})
}

// addBranch renderiza BRANCH: modelos de conectividade detalhados.
func (m *BlockMesher) addBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)

	connectivity := ""
	if ttype, ok := data.Tiletypes[tile.TileType]; ok {
		connectivity = ttype.Dir
	}

	modelName := "tree_branches"
	rotation := float32(0)

	if connectivity != "" && connectivity != "--------" {
		// Ordem esperada pelo models.json: NSEW
		n, s, e, w := connectivity[0] == 'N', connectivity[1] == 'S', connectivity[2] == 'E', connectivity[3] == 'W'
		suffix := ""
		if n {
			suffix += "n"
		}
		if s {
			suffix += "s"
		}
		if e {
			suffix += "e"
		}
		if w {
			suffix += "w"
		}

		if suffix != "" {
			modelName = "tree_branch_" + suffix
		} else {
			rotation = float32((coord.X*17+coord.Y*31)%4) * 90.0
		}
	} else {
		rotation = float32((coord.X*17 + coord.Y*31) % 360)
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.1, pos.Z - 0.5},
		Scale:       1.0,
		Rotation:    rotation,
		Color:       color,
	})
}

func (m *BlockMesher) addTwig(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)

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
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.1, pos.Z - 0.5},
		Scale:       1.0,
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

	modelName := "shrub" // BUSH.obj via config
	if tile.Shape() == dfproto.ShapeSapling {
		modelName = "sapling" // SAPLING.obj via config
	}

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
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

	// Otimização i7-900: Apenas 1 tufo de grama por tile para reduzir draw calls e vértices
	seed := int32(coord.X*7 + coord.Y*13)
	offX := float32(seed%10) / 10.0
	offZ := float32((seed/10)%10) / 10.0
	rotation := float32(seed % 360)

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "shrub", // Usa o modelo leve BUSH.obj
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + offX, pos.Y + 0.1, pos.Z - offZ},
		Scale:       0.15 + (float32(seed%5) / 20.0),
		Rotation:    rotation,
		Color:       color,
	})
}
