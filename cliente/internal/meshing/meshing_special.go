package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"fmt"
	"strings"
	"sync/atomic"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var rampDebugCount atomic.Int64

// addRamp renderiza a rampa usando o sistema de material para garantir shaders sólidos (Fase 34: Fix Final)
func (m *BlockMesher) addRamp(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	tile.CalculateRampType()
	rampType := int(tile.RampType)
	if rampType <= 0 || rampType > 26 {
		rampType = 1
	}

	modelName := fmt.Sprintf("ramps/RAMP_%d.obj", rampType)

	if m.AssetMgr != nil {
		entry := m.AssetMgr.GetTileMesh("RAMP:*:*:*:*")
		if entry != nil {
			sub := m.AssetMgr.GetSubObjectForRamp(entry, rampType)
			if sub != nil && sub.File != "" {
				modelName = sub.File
			}
		}
	}

	texName := m.MatStore.GetTextureName(tile.MaterialCategory())

	// Garantir que o nome do modelo seja consistente com o carregado no renderer.go
	// (Caso o AssetMgr não tenha retornado um nome específico do JSON)
	if !strings.Contains(modelName, "/") && !strings.HasPrefix(modelName, "ramps/") {
		modelName = strings.ToLower(modelName)
	}

	// Adicionamos como ModelInstance, mas vamos garantir que o Renderer NÃO use shaders de planta para ela
	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   modelName,
		TextureName: texName,
		Position: [3]float32{pos.X + 0.5, pos.Y, pos.Z - 0.5},
		Scale:    0.5,
		Color:    color,
		IsRamp:   true, // Nova flag para o Renderer
	})
}

func (m *BlockMesher) addStairs(coord util.DFCoord, shape dfproto.TiletypeShape, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	steps := 4
	stepH := float32(1.0) / float32(steps)
	stepD := d / float32(steps)

	for i := 0; i < steps; i++ {
		curH := float32(i+1) * stepH
		prevH := float32(i) * stepH
		curZ := z - float32(i)*stepD
		
		// Posição base do degrau
		stepPos := rl.Vector3{X: x, Y: y, Z: curZ}

		// Topo do degrau (sempre visível)
		// Usamos addQuadFace para ganhar AO e UVs automáticos
		m.addQuadFace(rl.Vector3{X: x, Y: y + prevH, Z: curZ}, w, stepD, stepH, util.DirUp, color, buffer, coord, data)

		// Frente do degrau (Vertical)
		m.addQuadFace(stepPos, w, stepD, curH, util.DirNorth, color, buffer, coord, data)
		
		// Laterais do degrau
		m.addQuadFace(stepPos, w, stepD, curH, util.DirWest, color, buffer, coord, data)
		m.addQuadFace(stepPos, w, stepD, curH, util.DirEast, color, buffer, coord, data)
	}

	// Face Inferior (se for escada que desce)
	if shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
		m.addQuadFace(rl.Vector3{X: x, Y: y, Z: z}, w, d, 1.0, util.DirDown, color, buffer, coord, data)
	}
}

func (m *BlockMesher) addFortification(coord util.DFCoord, color [4]uint8, buffer *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Base da fortificação (altura 0.4)
	m.addCubeGreedy(rl.Vector3{X: x, Y: y, Z: z}, w, 0.4, d, color, true, true, true, true, true, true, buffer, coord, data)
	
	// Topo da fortificação (altura 0.3, começando no 0.7)
	m.addCubeGreedy(rl.Vector3{X: x, Y: y + 0.7, Z: z}, w, 0.3, d, color, true, true, true, true, true, true, buffer, coord, data)
}

