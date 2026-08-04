package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"
)

// addTrunk lida com troncos de árvore e tocos (stumps)
func (m *BlockMesher) addTrunk(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	
	modelName := "stump"
	scale := float32(0.45)

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.05, pos.Z - 0.5},
		Scale:       scale,
		Rotation:    float32((coord.X*17 + coord.Y*11) % 360),
		Color:       color,
	})
}

func (m *BlockMesher) addBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {}
func (m *BlockMesher) addTwig(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore)   {}

// addShrub adiciona arbustos e mudas com variedade de modelos (Stonesense Style)
func (m *BlockMesher) addShrub(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	seed := int(coord.X*7 + coord.Y*13 + coord.Z*17)
	
	modelName := "shrub"
	scale := float32(0.4)

	// Lógica de seleção variada
	if tile.Shape() == dfproto.ShapeSapling {
		idx := (seed % 3) + 1
		modelName = fmt.Sprintf("foliage_%d", idx)
	} else if seed%5 == 0 {
		modelName = "fruit"
	} else if seed%3 == 0 {
		modelName = "foliage_3"
	}

	// Tint de cor aleatório
	finalColor := color
	tint := float32(0.95 + (float32(seed%10) / 100.0))
	finalColor[0] = uint8(float32(color[0]) * tint)
	finalColor[1] = uint8(float32(color[1]) * tint)
	finalColor[2] = uint8(float32(color[2]) * tint)

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y + 0.1, pos.Z - 0.5},
		Scale:       scale + (float32(seed%4) / 40.0),
		Rotation:    float32(seed % 360),
		Color:       finalColor,
	})
}

// addGrass adiciona grama variada (9 modelos)
func (m *BlockMesher) addGrass(coord util.DFCoord, tile *mapdata.Tile, res *Result) {
	pos := util.DFToWorldPos(coord)
	rlColor := m.MatStore.GetTileColor(tile)
	color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

	seed := int32(coord.X*13 + coord.Y*7 + coord.Z*31)
	
	// Seleção de modelos grass_01 até grass_09
	grassIdx := (seed % 9) + 1
	modelName := fmt.Sprintf("grass_%02d", grassIdx)

	// Tint da grama
	tint := float32(0.9 + (float32(seed%20) / 100.0))
	color[0] = uint8(float32(color[0]) * tint)
	color[1] = uint8(float32(color[1]) * tint)
	color[2] = uint8(float32(color[2]) * tint)

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: "tilemat_grass",
		Position:    [3]float32{pos.X + 0.4 + (float32(seed%3)/10.0), pos.Y + 0.05, pos.Z - 0.4 - (float32((seed/4)%3)/10.0)},
		Scale:       0.55 + (float32(seed%5) / 10.0),
		Rotation:    float32(seed % 360),
		Color:       color,
	})
}
