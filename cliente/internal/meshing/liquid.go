package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
)

// GenerateLiquidGeometry gera a malha para água ou magma em um tile.
func GenerateLiquidGeometry(tile *mapdata.Tile, buffer *MeshBuffer) {
	if tile.WaterLevel > 0 {
		// Encode level (1-7) into Alpha (0-255) for the shader to use as depth info
		alpha := uint8(float32(tile.WaterLevel) / 7.0 * 255)
		addLiquidPlane(tile, tile.WaterLevel, [4]uint8{20, 130, 220, alpha}, buffer)
	} else if tile.MagmaLevel > 0 {
		// Magma também pode usar o alpha para efeitos, embora seja mais opaco
		addLiquidPlane(tile, tile.MagmaLevel, [4]uint8{255, 50, 0, 255}, buffer)
	}
}

func addLiquidPlane(tile *mapdata.Tile, level int32, color [4]uint8, buffer *MeshBuffer) {
	pos := util.DFToWorldPos(tile.Position)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Função de ajuda local para buscar o nível de fluido em qualquer offset
	getFluidLevel := func(dx, dy int32) float32 {
		neighborPos := tile.Position.Add(util.DFCoord{X: dx, Y: dy, Z: 0})
		if neighbor := tile.GetStore().GetTile(neighborPos); neighbor != nil {
			// Usa o máximo entre água e magma para simplificar a malha caso encostem
			l := neighbor.WaterLevel
			if neighbor.MagmaLevel > l {
				l = neighbor.MagmaLevel
			}
			return float32(l) / 7.0
		}
		// Se não há bloco vizinho carregado, assume a altura original do nosso bloco para não deformar a borda do chunk
		return float32(level) / 7.0
	}

	// Calculando a elevação nas 4 QUINAS (Cantos do bloco na malha)
	nw := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(-1, 0) + getFluidLevel(-1, -1)) / 4.0
	ne := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(1, 0) + getFluidLevel(1, -1)) / 4.0
	sw := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(-1, 0) + getFluidLevel(-1, 1)) / 4.0
	se := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(1, 0) + getFluidLevel(1, 1)) / 4.0

	if upTile := tile.Up(); upTile != nil && (upTile.WaterLevel > 0 || upTile.MagmaLevel > 0) {
		nw, ne, sw, se = 1.0, 1.0, 1.0, 1.0
	}

	u := float32(tile.FlowVector.X)
	v := float32(-tile.FlowVector.Y)
	flowUV := [2]float32{u, v}

	// SMART OFFSETS: Evita que a água inclinada atravesse paredes de terra.
	off := float32(0.01)
	nwX, nwZ := x, z
	if getFluidLevel(-1, 0) == 0 || getFluidLevel(0, -1) == 0 { nwX += off; nwZ -= off }
	neX, neZ := x + w, z
	if getFluidLevel(1, 0) == 0 || getFluidLevel(0, -1) == 0 { neX -= off; neZ -= off }
	seX, seZ := x + w, z - d
	if getFluidLevel(1, 0) == 0 || getFluidLevel(0, 1) == 0 { seX -= off; seZ += off }
	swX, swZ := x, z - d
	if getFluidLevel(-1, 0) == 0 || getFluidLevel(0, 1) == 0 { swX += off; swZ += off }

	// Face SUPERIOR (Surface)
	buffer.AddFaceUV(
		[3]float32{nwX, y + nw, nwZ}, // NW
		[3]float32{neX, y + ne, neZ}, // NE
		[3]float32{seX, y + se, seZ}, // SE
		[3]float32{swX, y + sw, swZ}, // SW
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, 1, 0}, color,
	)

	// Face INFERIOR (Base)
	buffer.AddFaceUV(
		[3]float32{x, y, z},         // NW
		[3]float32{x, y, z - d},     // SW
		[3]float32{x + w, y, z - d}, // SE
		[3]float32{x + w, y, z},     // NE
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, -1, 0}, color,
	)

	// BORDAS/LATERAIS (Volume) com offset fixo de 0.01
	sideOff := float32(0.01)
	// Norte (+Z)
	if getFluidLevel(0, -1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x, y, z - sideOff},              // Base-NW
			[3]float32{x + w, y, z - sideOff},          // Base-NE
			[3]float32{x + w, y + ne, z - sideOff},     // Top-NE
			[3]float32{x, y + nw, z - sideOff},         // Top-NW
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, 1}, color,
		)
	}
	// Sul (-Z)
	if getFluidLevel(0, 1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z - d + sideOff},      // Base-SE
			[3]float32{x, y, z - d + sideOff},          // Base-SW
			[3]float32{x, y + sw, z - d + sideOff},     // Top-SW
			[3]float32{x + w, y + se, z - d + sideOff}, // Top-SE
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, -1}, color,
		)
	}
	// Oeste (-X)
	if getFluidLevel(-1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + sideOff, y, z - d},          // Base-SW
			[3]float32{x + sideOff, y, z},              // Base-NW
			[3]float32{x + sideOff, y + nw, z},         // Top-NW
			[3]float32{x + sideOff, y + sw, z - d},     // Top-SW
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{-1, 0, 0}, color,
		)
	}
	// Leste (+X)
	if getFluidLevel(1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w - sideOff, y, z},          // Base-NE
			[3]float32{x + w - sideOff, y, z - d},      // Base-SE
			[3]float32{x + w - sideOff, y + se, z - d}, // Top-SE
			[3]float32{x + w - sideOff, y + ne, z},     // Top-NE
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{1, 0, 0}, color,
		)
	}
}
