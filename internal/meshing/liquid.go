package meshing

import (
	"FortressVision/internal/mapdata"
	"FortressVision/internal/util"
)

// GenerateLiquidGeometry gera a malha para água ou magma em um tile.
func GenerateLiquidGeometry(tile *mapdata.Tile, buffer *MeshBuffer) {
	if tile.WaterLevel > 0 {
		addLiquidPlane(tile, tile.WaterLevel, [4]uint8{0, 100, 200, 180}, buffer)
	} else if tile.MagmaLevel > 0 {
		addLiquidPlane(tile, tile.MagmaLevel, [4]uint8{255, 50, 0, 255}, buffer)
	}
}

func addLiquidPlane(tile *mapdata.Tile, level int32, color [4]uint8, buffer *MeshBuffer) {
	pos := util.DFToWorldPos(tile.Position)

	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Altura proporcional ao nível (1-7)
	// DFHack níveis 0-7. 7 é cheio (altura 1.0)
	h := float32(level) / 7.0

	// TODO: Suavização de superfície (médias de alturas dos vizinhos) - Futuro

	// Apenas o topo do líquido por enquanto
	// Face SUPERIOR (Sentido Anti-Horário para apontar para CIMA corretamente)
	// v1(0,0) -> v4(1,0) -> v3(1,-1) -> v2(0,-1)
	// U = (1,0,0), V = (0,0,-1). U x V = (0, 1, 0) -> Normal +Y
	buffer.AddFace(
		[3]float32{x, y + h, z},
		[3]float32{x + w, y + h, z},
		[3]float32{x + w, y + h, z - d},
		[3]float32{x, y + h, z - d},
		[3]float32{0, 1, 0}, color,
	)

	// Face INFERIOR (Sentido Horário, aponta para BAIXO)
	// Para ser visível de baixo da água
	buffer.AddFace(
		[3]float32{x, y + h, z},
		[3]float32{x, y + h, z - d},
		[3]float32{x + w, y + h, z - d},
		[3]float32{x + w, y + h, z},
		[3]float32{0, -1, 0}, color,
	)

	// Se o líquido não for cheio, talvez devêssemos desenhar faces laterais?
	// No Armok Vision, eles geralmente só desenham o topo se houver vizinho vazio lateralmente.
	// Por simplicidade inicial, apenas o topo.
}
