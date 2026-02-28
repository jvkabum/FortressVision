package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// addRamp renderiza a rampa usando o sistema de material para garantir shaders sólidos (Fase 34: Fix Final)
func (m *BlockMesher) addRamp(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	tile.CalculateRampType()
	rampType := int(tile.RampType)
	if rampType <= 0 || rampType > 26 {
		rampType = 1
	}

	modelName := fmt.Sprintf("ramp_%d", rampType)

	if m.AssetMgr != nil {
		entry := m.AssetMgr.GetTileMesh("RAMP:*:*:*:*")
		if entry != nil {
			sub := m.AssetMgr.GetSubObjectForRamp(entry, rampType)
			if sub != nil && sub.File != "" {
				modelName = sub.File
			}
		}
	}

	// Adicionamos como ModelInstance, mas vamos garantir que o Renderer NÃO use shaders de planta para ela
	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y, pos.Z - 0.5},
		Scale:       1.0,
		Color:       color,
		IsRamp:      true, // Nova flag para o Renderer
	})
}

func (m *BlockMesher) addStairs(coord util.DFCoord, shape dfproto.TiletypeShape, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	steps := 4
	stepH := float32(1.0) / float32(steps)
	stepD := d / float32(steps)
	sideColor := [4]uint8{uint8(float32(color[0]) * 0.8), uint8(float32(color[1]) * 0.8), uint8(float32(color[2]) * 0.8), color[3]}

	for i := 0; i < steps; i++ {
		curH := float32(i+1) * stepH
		prevH := float32(i) * stepH
		curZ := z - float32(i)*stepD

		buffer.AddFaceAOStandard(
			[3]float32{x, y + curH, curZ}, color,
			[3]float32{x, y + curH, curZ - stepD}, color,
			[3]float32{x + w, y + curH, curZ - stepD}, color,
			[3]float32{x + w, y + curH, curZ}, color,
			[3]float32{0, 1, 0},
		)

		buffer.AddFace(
			[3]float32{x, y + prevH, curZ},
			[3]float32{x + w, y + prevH, curZ},
			[3]float32{x + w, y + curH, curZ},
			[3]float32{x, y + curH, curZ},
			[3]float32{0, 0, 1}, sideColor,
		)

		buffer.AddFace(
			[3]float32{x, y + prevH, curZ - stepD},
			[3]float32{x, y + prevH, curZ},
			[3]float32{x, y + curH, curZ},
			[3]float32{x, y + curH, curZ - stepD},
			[3]float32{-1, 0, 0}, sideColor,
		)
		buffer.AddFace(
			[3]float32{x + w, y, curZ},
			[3]float32{x + w, y, curZ - stepD},
			[3]float32{x + w, y + curH, curZ - stepD},
			[3]float32{x + w, y + curH, curZ},
			[3]float32{1, 0, 0}, sideColor,
		)
	}

	if shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
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

	m.addCubeGreedy(rl.Vector3{X: x, Y: y, Z: z}, w, 0.4, d, color, false, true, true, true, true, true, buffer, coord, data)
	m.addCubeGreedy(rl.Vector3{X: x, Y: y + 0.7, Z: z}, w, 0.3, d, color, true, false, true, true, true, true, buffer, coord, data)

	buffer.AddFace(
		[3]float32{x, y + 0.7, z},
		[3]float32{x + w, y + 0.7, z},
		[3]float32{x + w, y + 0.7, z - d},
		[3]float32{x, y + 0.7, z - d},
		[3]float32{0, -1, 0}, color,
	)
	buffer.AddFace(
		[3]float32{x, y + 0.4, z},
		[3]float32{x, y + 0.4, z - d},
		[3]float32{x + w, y + 0.4, z - d},
		[3]float32{x + w, y + 0.4, z},
		[3]float32{0, 1, 0}, color,
	)
}
