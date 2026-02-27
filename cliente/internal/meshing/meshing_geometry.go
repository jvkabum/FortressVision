package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (m *BlockMesher) emitGreedyQuad(req Request, x, y, w, h int32, face util.Directions, tile *mapdata.Tile, getBuffer func(string) *MeshBuffer) {
	// Converte coordenadas locais (0-15) + dimensões (W, H) para mundo real
	pos := util.DFToWorldPos(util.DFCoord{X: req.Origin.X + x, Y: req.Origin.Y + y, Z: req.Origin.Z})

	// Espessura e formato baseados no shape
	shape := tile.Shape()
	thickness := float32(1.0)
	if shape == dfproto.ShapeFloor {
		thickness = 0.1
	}

	// Identificar material e cor
	texName := m.MatStore.GetTextureName(tile.MaterialCategory())
	targetBuffer := getBuffer(texName)
	rlColor := m.MatStore.GetTileColor(tile)
	color := [4]uint8{rlColor.R, rlColor.G, rlColor.B, rlColor.A}

	// Chamar o gerador de faces especializado (passando dimensões)
	m.addQuadFace(pos, float32(w), float32(h), thickness, face, color, targetBuffer, util.DFCoord{X: req.Origin.X + x, Y: req.Origin.Y + y, Z: req.Origin.Z}, req.Data)
}

func (m *BlockMesher) addQuadFace(pos rl.Vector3, w, h, thickness float32, face util.Directions, color [4]uint8, buffer *MeshBuffer, coord util.DFCoord, data *mapdata.MapDataStore) {
	px, py, pz := pos.X, pos.Y, pos.Z

	// Cores com AO por vértice
	// Recuperamos o padrao de AO para os cantos deste retângulo
	// Nota: Como garantimos no Greedy Mesher que todos os tiles do quad têm o mesmo AO,
	// podemos pegar o AO de qualquer tile do quad (neste caso, o tile na origem 'coord').
	c1, c2, c3, c4 := m.getQuadCornerColors(coord, face, color, data)

	switch face {
	case util.DirUp:
		buffer.AddFaceUVStandard(
			[3]float32{px, py + thickness, pz}, [2]float32{px, -pz}, c1,
			[3]float32{px + w, py + thickness, pz}, [2]float32{px + w, -pz}, c2,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{px + w, -(pz - h)}, c3,
			[3]float32{px, py + thickness, pz - h}, [2]float32{px, -(pz - h)}, c4,
			[3]float32{0, 1, 0},
		)
	case util.DirDown:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{px, -pz}, c1,
			[3]float32{px, py, pz - h}, [2]float32{px, -(pz - h)}, c2,
			[3]float32{px + w, py, pz - h}, [2]float32{px + w, -(pz - h)}, c3,
			[3]float32{px + w, py, pz}, [2]float32{px + w, -pz}, c4,
			[3]float32{0, -1, 0},
		)
	case util.DirNorth:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{px, -py}, c1,
			[3]float32{px + w, py, pz}, [2]float32{px + w, -py}, c2,
			[3]float32{px + w, py + thickness, pz}, [2]float32{px + w, -(py + thickness)}, c3,
			[3]float32{px, py + thickness, pz}, [2]float32{px, -(py + thickness)}, c4,
			[3]float32{0, 0, 1},
		)
	case util.DirSouth:
		buffer.AddFaceUVStandard(
			[3]float32{px + w, py, pz - h}, [2]float32{px + w, -py}, c1,
			[3]float32{px, py, pz - h}, [2]float32{px, -py}, c2,
			[3]float32{px, py + thickness, pz - h}, [2]float32{px, -(py + thickness)}, c3,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{px + w, -(py + thickness)}, c4,
			[3]float32{0, 0, -1},
		)
	case util.DirWest:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz - h}, [2]float32{-pz + h, -py}, c1,
			[3]float32{px, py, pz}, [2]float32{-pz, -py}, c2,
			[3]float32{px, py + thickness, pz}, [2]float32{-pz, -(py + thickness)}, c3,
			[3]float32{px, py + thickness, pz - h}, [2]float32{-pz + h, -(py + thickness)}, c4,
			[3]float32{-1, 0, 0},
		)
	case util.DirEast:
		buffer.AddFaceUVStandard(
			[3]float32{px + w, py, pz}, [2]float32{-pz, -py}, c1,
			[3]float32{px + w, py, pz - h}, [2]float32{-pz + h, -py}, c2,
			[3]float32{px + w, py + thickness, pz - h}, [2]float32{-pz + h, -(py + thickness)}, c3,
			[3]float32{px + w, py + thickness, pz}, [2]float32{-pz, -(py + thickness)}, c4,
			[3]float32{1, 0, 0},
		)
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

		buffer.AddFaceUVStandard(
			[3]float32{x, y + h, z}, [2]float32{x, -z}, applyAO(color, aoNW),
			[3]float32{x + w, y + h, z}, [2]float32{x + w, -z}, applyAO(color, aoNE),
			[3]float32{x + w, y + h, z - d}, [2]float32{x + w, -(z - d)}, applyAO(color, aoSE),
			[3]float32{x, y + h, z - d}, [2]float32{x, -(z - d)}, applyAO(color, aoSW),
			[3]float32{0, 1, 0},
		)
	}

	// Face Baixo (-Y)
	if drawDown {
		buffer.AddFaceUV(
			[3]float32{x, y, z}, [3]float32{x, y, z - d}, [3]float32{x + w, y, z - d}, [3]float32{x + w, y, z},
			[2]float32{x, -z}, [2]float32{x, -(z - d)}, [2]float32{x + w, -(z - d)}, [2]float32{x + w, -z},
			[3]float32{0, -1, 0}, color,
		)
	}

	// Face Norte (+Z)
	if drawNorth {
		buffer.AddFaceUV(
			[3]float32{x, y, z}, [3]float32{x + w, y, z}, [3]float32{x + w, y + h, z}, [3]float32{x, y + h, z},
			[2]float32{x, -y}, [2]float32{x + w, -y}, [2]float32{x + w, -(y + h)}, [2]float32{x, -(y + h)},
			[3]float32{0, 0, 1}, color,
		)
	}

	// Face Sul (-Z)
	if drawSouth {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z - d}, [3]float32{x, y, z - d}, [3]float32{x, y + h, z - d}, [3]float32{x + w, y + h, z - d},
			[2]float32{x + w, -y}, [2]float32{x, -y}, [2]float32{x, -(y + h)}, [2]float32{x + w, -(y + h)},
			[3]float32{0, 0, -1}, color,
		)
	}

	// Face Oeste (-X)
	if drawWest {
		buffer.AddFaceUV(
			[3]float32{x, y, z - d}, [3]float32{x, y, z}, [3]float32{x, y + h, z}, [3]float32{x, y + h, z - d},
			[2]float32{-z + d, -y}, [2]float32{-z, -y}, [2]float32{-z, -(y + h)}, [2]float32{-z + d, -(y + h)},
			[3]float32{-1, 0, 0}, color,
		)
	}

	// Face Leste (+X)
	if drawEast {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z}, [3]float32{x + w, y, z - d}, [3]float32{x + w, y + h, z - d}, [3]float32{x + w, y + h, z},
			[2]float32{-z, -y}, [2]float32{-z + d, -y}, [2]float32{-z + d, -(y + h)}, [2]float32{-z, -(y + h)},
			[3]float32{1, 0, 0}, color,
		)
	}
}
