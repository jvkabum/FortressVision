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

func (m *BlockMesher) addQuadFace(pos rl.Vector3, sizeX, sizeZ, thickness float32, face util.Directions, color [4]uint8, buffer *MeshBuffer, coord util.DFCoord, data *mapdata.MapDataStore) {
	px, py, pz := pos.X, pos.Y, pos.Z

	// getQuadCornerColors agora deve retornar cores de AO para qualquer face
	c1, c2, c3, c4 := m.getQuadCornerColors(coord, face, color, data)

	switch face {
	case util.DirUp:
		// Horizontal: X=sizeX, Z=sizeZ. Vertical: Y=thickness
		buffer.AddFaceUVStandard(
			[3]float32{px, py + thickness, pz}, [2]float32{0, sizeZ}, c1,
			[3]float32{px + sizeX, py + thickness, pz}, [2]float32{sizeX, sizeZ}, c2,
			[3]float32{px + sizeX, py + thickness, pz - sizeZ}, [2]float32{sizeX, 0}, c3,
			[3]float32{px, py + thickness, pz - sizeZ}, [2]float32{0, 0}, c4,
			[3]float32{0, 1, 0},
		)
	case util.DirDown:
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{0, sizeZ}, c1,
			[3]float32{px, py, pz - sizeZ}, [2]float32{0, 0}, c2,
			[3]float32{px + sizeX, py, pz - sizeZ}, [2]float32{sizeX, 0}, c3,
			[3]float32{px + sizeX, py, pz}, [2]float32{sizeX, sizeZ}, c4,
			[3]float32{0, -1, 0},
		)
	case util.DirNorth:
		// North: Frente do bloco (+Z). Horizontal=X (sizeX), Vertical=Y (thickness)
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz}, [2]float32{0, thickness}, c1,
			[3]float32{px + sizeX, py, pz}, [2]float32{sizeX, thickness}, c2,
			[3]float32{px + sizeX, py + thickness, pz}, [2]float32{sizeX, 0}, c3,
			[3]float32{px, py + thickness, pz}, [2]float32{0, 0}, c4,
			[3]float32{0, 0, 1},
		)
	case util.DirSouth:
		// South: Trás do bloco (-Z). pz-sizeZ é o offset horizontal. 
		// Horizontal=X (sizeX), Vertical=Y (thickness)
		buffer.AddFaceUVStandard(
			[3]float32{px + sizeX, py, pz - sizeZ}, [2]float32{0, thickness}, c1,
			[3]float32{px, py, pz - sizeZ}, [2]float32{sizeX, thickness}, c2,
			[3]float32{px, py + thickness, pz - sizeZ}, [2]float32{sizeX, 0}, c3,
			[3]float32{px + sizeX, py + thickness, pz - sizeZ}, [2]float32{0, 0}, c4,
			[3]float32{0, 0, -1},
		)
	case util.DirWest:
		// West: Lado Esquerdo (-X). pz-sizeZ é a extensão horizontal.
		// Horizontal=Z (sizeZ), Vertical=Y (thickness)
		buffer.AddFaceUVStandard(
			[3]float32{px, py, pz - sizeZ}, [2]float32{0, thickness}, c1,
			[3]float32{px, py, pz}, [2]float32{sizeZ, thickness}, c2,
			[3]float32{px, py + thickness, pz}, [2]float32{sizeZ, 0}, c3,
			[3]float32{px, py + thickness, pz - sizeZ}, [2]float32{0, 0}, c4,
			[3]float32{-1, 0, 0},
		)
	case util.DirEast:
		// East: Lado Direito (+X). px+sizeX é a borda.
		// Horizontal=Z (sizeZ), Vertical=Y (thickness)
		buffer.AddFaceUVStandard(
			[3]float32{px + sizeX, py, pz}, [2]float32{0, thickness}, c1,
			[3]float32{px + sizeX, py, pz - sizeZ}, [2]float32{sizeZ, thickness}, c2,
			[3]float32{px + sizeX, py + thickness, pz - sizeZ}, [2]float32{sizeZ, 0}, c3,
			[3]float32{px + sizeX, py + thickness, pz}, [2]float32{0, 0}, c4,
			[3]float32{1, 0, 0},
		)
	}
}

// addCubeGreedy adiciona um cubo com culling de faces previamente calculado.
// Delega cada face ao addQuadFace universal (AO + UVs corretos).
//
// Parâmetros:
//   pos   = canto inferior-noroeste do cubo (ponto base)
//   w     = largura no eixo X
//   h     = altura no eixo Y (thickness das faces)
//   d     = profundidade no eixo Z
func (m *BlockMesher) addCubeGreedy(pos rl.Vector3, w, h, d float32, color [4]uint8,
	drawUp, drawDown, drawNorth, drawSouth, drawWest, drawEast bool, buffer *MeshBuffer, coord util.DFCoord, data *mapdata.MapDataStore) {

	// Para TODAS as faces, a "espessura" visível do bloco é a altura h.
	// addQuadFace internamente soma thickness ao Y para calcular o topo.
	if drawUp {
		m.addQuadFace(pos, w, d, h, util.DirUp, color, buffer, coord, data)
	}
	if drawDown {
		m.addQuadFace(pos, w, d, h, util.DirDown, color, buffer, coord, data)
	}
	if drawNorth {
		m.addQuadFace(pos, w, d, h, util.DirNorth, color, buffer, coord, data)
	}
	if drawSouth {
		m.addQuadFace(pos, w, d, h, util.DirSouth, color, buffer, coord, data)
	}
	if drawWest {
		m.addQuadFace(pos, w, d, h, util.DirWest, color, buffer, coord, data)
	}
	if drawEast {
		m.addQuadFace(pos, w, d, h, util.DirEast, color, buffer, coord, data)
	}
}
