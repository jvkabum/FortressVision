package liquid

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"log"
)

// ========================================================================
// SHADERS DE ÁGUA
// ========================================================================

// WaterVertexShader é o vertex shader GLSL para efeitos de superfície d'água.
const WaterVertexShader = `
#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec3 vertexNormal;
in vec4 vertexColor;
uniform mat4 mvp;
out vec4 fragColor;
void main() {
    fragColor = vertexColor;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}
`

// WaterFragmentShader é o fragment shader GLSL para efeitos visuais de água.
const WaterFragmentShader = `
#version 330
in vec4 fragColor;
out vec4 finalColor;
void main()
{
    finalColor = fragColor;
}
`

// ========================================================================
// GERAÇÃO DE GEOMETRIA DE ÁGUA
// ========================================================================

// GenerateWaterGeometry gera a malha para água em um tile.
func GenerateWaterGeometry(tile *mapdata.Tile, buffer GeometryBuffer) {
	log.Printf("[Liquid/Water] Gerando água para %v (Level:%d)", tile.Position, tile.WaterLevel)
	if tile.WaterLevel > 0 {
		// Azul translúcido para água
		color := [4]uint8{0, 150, 255, 160}
		addWaterPlane(tile, tile.WaterLevel, color, buffer)
		TraceGeometry(tile.Position.X, tile.Position.Y, tile.Position.Z, int32(tile.WaterLevel), 6)
	}
}

func addWaterPlane(tile *mapdata.Tile, level int32, color [4]uint8, buffer GeometryBuffer) {
	pos := util.DFToWorldPos(tile.Position)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Altura da água + Pequeno Offset para evitar Z-Fighting com o fundo
	h := (float32(level) / 7.0) + 0.05

	// Flow UV nulo para debug
	flowUV := [2]float32{0, 0}

	// Face SUPERIOR (NW -> NE -> SE -> SW) - Ordem CLOCKWISE (CW)
	buffer.AddFaceUV(
		[3]float32{x, y + h, z},         // NW
		[3]float32{x + w, y + h, z},     // NE
		[3]float32{x + w, y + h, z - d}, // SE
		[3]float32{x, y + h, z - d},     // SW
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, 1, 0}, color,
	)
}
