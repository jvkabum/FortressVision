package meshing

import (
	"FortressVision/cliente/internal/treegen"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"math"
)

// addTrunk lida com troncos de árvore usando geometria procedural simples.
func (m *BlockMesher) addTrunk(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)

	// Raio base do tronco
	radius := float32(0.4)
	if tile.TrunkPercent > 0 {
		radius = float32(tile.TrunkPercent) / 200.0
	}
	if radius < 0.1 { radius = 0.1 }

	// Tentar suavizar com o bloco de cima para conectividade
	endRad := radius
	if up := data.GetTile(util.DFCoord{X: coord.X, Y: coord.Y, Z: coord.Z + 1}); up != nil {
		if up.Shape() == 138 || up.Shape() == 139 { // TrunkBranch ou Branch
			endRad = float32(up.TrunkPercent) / 200.0
			if endRad < 0.1 {
				endRad = 0.1
			}
		}
	}

	// Cilindro vertical que preenche o bloco
	start := util.Vector3{X: pos.X + 0.5, Y: pos.Y, Z: pos.Z - 0.5}
	end := util.Vector3{X: pos.X + 0.5, Y: pos.Y + 1.0, Z: pos.Z - 0.5}

	trunkColor := treegen.GetTrunkColor(tile.PlantID, color, int(coord.Z), radius)
	mesh := treegen.GenerateCylinder(start, end, radius, endRad, 12, false)
	defer treegen.PutMeshData(mesh)

	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)
	m.addTreeMeshToBuffer(mesh, trunkColor, buf)
}

// addBranch desenha um galho simples como um cilindro inclinado.
func (m *BlockMesher) addBranch(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	radius := float32(0.15)
	
	// Direção determinística
	angle := float64((int(coord.X)*13 + int(coord.Y)*7 + int(coord.Z)*31) % 360)
	rad := angle * math.Pi / 180.0
	dirX := float32(math.Cos(rad))
	dirZ := float32(math.Sin(rad))
	
	// Conectar galho ao tronco central (nascer da borda do tronco)
	start := util.Vector3{X: pos.X + 0.5, Y: pos.Y + 0.5, Z: pos.Z - 0.5} // Default start in center
	
	// Try to find a nearby trunk to connect to
	if trunkPos, trunkRad, found := m.findNeighborTrunk(coord, data); found {
		// Calculate direction from trunk center to branch start
		trunkCenter := util.Vector3{X: float32(trunkPos.X) + 0.5, Y: float32(trunkPos.Y) + 0.5, Z: float32(trunkPos.Z) - 0.5}
		
		// Adjust start point to be on the edge of the trunk, pointing towards the branch direction
		// We want the branch to originate from the trunk's surface.
		// The branch direction is (dirX, 0, dirZ) relative to the current block's center.
		// We need to find a point on the trunk's circumference that is "closest" to the branch's direction.
		
		// For simplicity, let's just offset from the trunk's center in the branch's direction
		// by the trunk's radius.
		start = util.Vector3{
			X: trunkCenter.X + dirX * trunkRad,
			Y: trunkCenter.Y + 0.2, // Slightly above trunk center
			Z: trunkCenter.Z + dirZ * trunkRad,
		}
	}

	end := util.Vector3{X: pos.X + 0.5 + dirX*0.7, Y: pos.Y + 0.8, Z: pos.Z - 0.5 + dirZ*0.7}

	branchColor := treegen.GetTrunkColor(tile.PlantID, color, int(coord.Z), radius)
	mesh := treegen.GenerateCylinder(start, end, radius, radius*0.6, 8, false)
	defer treegen.PutMeshData(mesh)

	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)
	m.addTreeMeshToBuffer(mesh, branchColor, buf)
}

// addTwig decide se desenha folhas ou um graveto pequeno.
func (m *BlockMesher) addTwig(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, res *Result, data *mapdata.MapDataStore) {
	isLeaf := tile.Shape() == 137 || tile.Shape() == 143 // Twig/Leaf shapes
	if isLeaf {
		m.addLeafCluster(coord, tile, color, getBuffer, data)
	} else {
		pos := util.DFToWorldPos(coord)
		start := util.Vector3{X: pos.X + 0.5, Y: pos.Y + 0.2, Z: pos.Z - 0.5}
		end := util.Vector3{X: pos.X + 0.5, Y: pos.Y + 0.6, Z: pos.Z - 0.5}
		mesh := treegen.GenerateCylinder(start, end, 0.05, 0.03, 4, false)
		defer treegen.PutMeshData(mesh)

		texName := m.MatStore.GetTextureName(tile.MaterialCategory())
		buf := getBuffer(texName)
		m.addTreeMeshToBuffer(mesh, color, buf)
	}
}

// addLeafCluster desenha folhagem usando modelos cruzados.
func (m *BlockMesher) addLeafCluster(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, getBuffer func(string) *MeshBuffer, data *mapdata.MapDataStore) {
	pos := util.DFToWorldPos(coord)
	center := util.Vector3{X: pos.X + 0.5, Y: pos.Y + 0.5, Z: pos.Z - 0.5}
	dist := float32(1.0) // Simplificado para reset
	leafColor, _ := treegen.GetLeafArt(tile.PlantID, color, dist)

	lType := treegen.LeafTypeCross
	if tile.PlantID == 42 { lType = treegen.LeafTypeDisc }

	mesh := treegen.GenerateLeafCluster(center, lType, 1.2)
	defer treegen.PutMeshData(mesh)

	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	buf := getBuffer(texName)
	m.addTreeMeshToBuffer(mesh, leafColor, buf)
}

// addShrub desenha arbustos ou plantas pequenas.
func (m *BlockMesher) addShrub(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	seed := int(coord.X*31 + coord.Y*17 + coord.Z*7)
	offX := float32(seed%10) / 10.0
	offZ := float32((seed/10)%10) / 10.0

	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "shrub",
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + offX, pos.Y + 0.1, pos.Z - offZ},
		Scale:       0.5 + (float32(seed%5) / 10.0),
		Rotation:    float32(seed%360),
		Color:       color,
	})
}

// addGrass desenha grama.
func (m *BlockMesher) addGrass(coord util.DFCoord, tile *mapdata.Tile, color [4]uint8, res *Result) {
	pos := util.DFToWorldPos(coord)
	res.ModelInstances = append(res.ModelInstances, ModelInstance{
		ModelName:   "grass",
		TextureName: m.MatStore.GetTextureName(tile.MaterialCategory()),
		Position:    [3]float32{pos.X + 0.5, pos.Y, pos.Z - 0.5},
		Scale:       1.0,
		Color:       color,
	})
}

func (m *BlockMesher) addTreeMeshToBuffer(mesh *treegen.MeshData, color [4]uint8, buffer *MeshBuffer) {
	if buffer == nil || mesh == nil { return }
	startIdx := uint16(len(buffer.Geometry.Vertices) / 3)
	for i := 0; i < len(mesh.Vertices); i += 8 {
		v := [3]float32{mesh.Vertices[i], mesh.Vertices[i+1], mesh.Vertices[i+2]}
		n := [3]float32{mesh.Vertices[i+3], mesh.Vertices[i+4], mesh.Vertices[i+5]}
		uv := [2]float32{mesh.Vertices[i+6], mesh.Vertices[i+7]}
		buffer.addVertexUV(v, uv, n, color)
	}
	for _, idx := range mesh.Indices {
		buffer.Geometry.Indices = append(buffer.Geometry.Indices, startIdx+uint16(idx))
	}
}

// findNeighborTrunk procura um tronco nos blocos adjacentes para ancorar o galho.
func (m *BlockMesher) findNeighborTrunk(coord util.DFCoord, data *mapdata.MapDataStore) (util.DFCoord, float32, bool) {
	// Verificar as 4 direções cardinais (mesmo Z)
	dirs := []util.DFCoord{
		{X: coord.X + 1, Y: coord.Y, Z: coord.Z},
		{X: coord.X - 1, Y: coord.Y, Z: coord.Z},
		{X: coord.X, Y: coord.Y + 1, Z: coord.Z},
		{X: coord.X, Y: coord.Y - 1, Z: coord.Z},
	}
	for _, d := range dirs {
		if t := data.GetTile(d); t != nil {
			if t.Shape() == 138 { // TrunkBranch
				radius := float32(0.4)
				if t.TrunkPercent > 0 {
					radius = float32(t.TrunkPercent) / 200.0
				}
				return d, radius, true
			}
		}
	}
	return coord, 0, false
}
