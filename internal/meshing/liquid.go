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
	// Para unir perfeitamente com os vizinhos, a altura de cada quina é a MÉDIA das alturas dos 4 blocos que tocam naquilo.
	// Note: O jogo DF tem água descendo como cachoeira, mas o level varia de 0 a 7. 0 = não renderiza plane normal.
	// As coordenadas cartesianas: X cresce (Leste), Z diminui (-d) (Sul). DF Y+ = 3D Z-

	// Quina NW (Noroeste): Bloco atual, Norte, Oeste, Noroeste (X, Z)
	nw := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(-1, 0) + getFluidLevel(-1, -1)) / 4.0
	// Quina NE (Nordeste): Bloco atual, Norte, Leste, Nordeste (X+W, Z)
	ne := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(1, 0) + getFluidLevel(1, -1)) / 4.0
	// Quina SW (Sudoeste): Bloco atual, Sul, Oeste, Sudoeste (X, Z-D)
	sw := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(-1, 0) + getFluidLevel(-1, 1)) / 4.0
	// Quina SE (Sudeste): Bloco atual, Sul, Leste, Sudeste (X+W, Z-D)
	se := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(1, 0) + getFluidLevel(1, 1)) / 4.0

	// Se for o nível máximo 7 e o bloco de "cima/teto" tiver água, as quinas cravam no 1.0 para ligar cachoeiras
	if upTile := tile.Up(); upTile != nil && (upTile.WaterLevel > 0 || upTile.MagmaLevel > 0) {
		// Simplificação pra líquidos transbordantes e colunas de água
		nw, ne, sw, se = 1.0, 1.0, 1.0, 1.0
	}

	// Transformamos o Vetor Direcional do DFHack em UV para ser resgatado na GPU pelo Shader
	u := float32(tile.FlowVector.X)
	v := float32(-tile.FlowVector.Y) // No motor 3D, Z escala negativamente com DF Y
	flowUV := [2]float32{u, v}

	// Face SUPERIOR COM RAMPAS SUAVES (Continuous Surface Mesh)
	// v1(NW) -> v4(NE) -> v3(SE) -> v2(SW)   --- Ordem ccw apontando +Y
	buffer.AddFaceUV(
		[3]float32{x, y + nw, z},         // NW
		[3]float32{x + w, y + ne, z},     // NE
		[3]float32{x + w, y + se, z - d}, // SE
		[3]float32{x, y + sw, z - d},     // SW
		flowUV, flowUV, flowUV, flowUV,   // Dados de direção global do Voxel na GPU (Flow)
		[3]float32{0, 1, 0}, color, // Normal
	)

	// Face INFERIOR plana (Sem inclinações drásticas para facilitar visual debaixo d'água) - CCW
	buffer.AddFaceUV(
		[3]float32{x, y, z},         // NW
		[3]float32{x, y, z - d},     // SW
		[3]float32{x + w, y, z - d}, // SE
		[3]float32{x + w, y, z},     // NE
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, -1, 0}, color,
	)

	// BORDAS/LATERAIS DA ÁGUA (Waterfall/Volume)
	// Se o nível de fluido cair drasticamente em relação aos vizinhos, desenhamos as faces verticais.
	// Norte (+Z)
	if getFluidLevel(0, -1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x, y, z},          // Base-NW
			[3]float32{x + w, y, z},      // Base-NE
			[3]float32{x + w, y + ne, z}, // Top-NE
			[3]float32{x, y + nw, z},     // Top-NW
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, 1}, color,
		)
	}
	// Sul (-Z)
	if getFluidLevel(0, 1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z - d},      // Base-SE
			[3]float32{x, y, z - d},          // Base-SW
			[3]float32{x, y + sw, z - d},     // Top-SW
			[3]float32{x + w, y + se, z - d}, // Top-SE
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, -1}, color,
		)
	}
	// Oeste (-X)
	if getFluidLevel(-1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x, y, z - d},      // Base-SW
			[3]float32{x, y, z},          // Base-NW
			[3]float32{x, y + nw, z},     // Top-NW
			[3]float32{x, y + sw, z - d}, // Top-SW
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{-1, 0, 0}, color,
		)
	}
	// Leste (+X)
	if getFluidLevel(1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z},          // Base-NE
			[3]float32{x + w, y, z - d},      // Base-SE
			[3]float32{x + w, y + se, z - d}, // Top-SE
			[3]float32{x + w, y + ne, z},     // Top-NE
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{1, 0, 0}, color,
		)
	}
}
