package liquid

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
)

// ========================================================================
// SHADERS DE LAVA (futuramente customizáveis, por ora reusa lógica simples)
// ========================================================================

// LavaVertexShader é o vertex shader GLSL para lava/magma.
// Lava tem menos ondulação e mais brilho emissivo.
const LavaVertexShader = `
#version 330

in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec3 vertexNormal;
in vec4 vertexColor;

uniform mat4 mvp;
uniform float time;

out vec2 fragTexCoord;
out vec4 fragColor;
out vec3 fragNormal;
out vec3 fragWorldPos;

void main()
{
    fragTexCoord = vertexPosition.xz + vec2(time * 0.1, time * 0.05);
    fragColor = vertexColor;
    fragNormal = vertexNormal;

    vec3 pos = vertexPosition;
    // Lava ondula muito menos que água
    pos.y += sin(pos.x * 2.0 + time * 0.5) * 0.02;
    pos.y += sin(pos.z * 1.5 + time * 0.3) * 0.015;

    fragWorldPos = pos;
    gl_Position = mvp * vec4(pos, 1.0);
}
`

// LavaFragmentShader é o fragment shader GLSL para efeitos visuais de lava.
const LavaFragmentShader = `
#version 330

in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;
in vec3 fragWorldPos;

uniform float time;
uniform vec3 camPos;

out vec4 finalColor;

float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float a = hash(i);
    float b = hash(i + vec2(1.0, 0.0));
    float c = hash(i + vec2(0.0, 1.0));
    float d = hash(i + vec2(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

void main()
{
    // Cores de magma
    vec3 hotColor  = vec3(1.0, 0.6, 0.0);  // Laranja incandescente
    vec3 coolColor = vec3(0.6, 0.1, 0.0);  // Vermelho escuro

    // Padrão de fluxo procedural
    vec2 uv = fragTexCoord;
    float n1 = noise(uv * 3.0 + time * 0.2);
    float n2 = noise(uv * 5.0 - time * 0.15);
    float pattern = (n1 + n2) * 0.5;

    vec3 baseColor = mix(coolColor, hotColor, pattern);

    // Veias brilhantes de magma
    float veins = smoothstep(0.45, 0.55, pattern);
    baseColor = mix(baseColor, vec3(1.0, 0.9, 0.3), veins * 0.4);

    // Emissão (lava sempre brilha)
    float emission = 0.3 + 0.2 * pattern;
    baseColor *= (1.0 + emission);

    // Lava é quase completamente opaca
    float alpha = 0.95;

    finalColor = vec4(baseColor, alpha);
}
`

// ========================================================================
// GERAÇÃO DE GEOMETRIA DE LAVA
// ========================================================================

// GenerateLavaGeometry gera a malha para magma/lava em um tile.
func GenerateLavaGeometry(tile *mapdata.Tile, buffer GeometryBuffer) {
	if tile.MagmaLevel > 0 {
		// Magma é mais opaco que água
		addLavaPlane(tile, tile.MagmaLevel, [4]uint8{255, 100, 0, 240}, buffer)
	}
}

func addLavaPlane(tile *mapdata.Tile, level int32, color [4]uint8, buffer GeometryBuffer) {
	pos := util.DFToWorldPos(tile.Position)
	x, y, z := pos.X, pos.Y, pos.Z
	w, d := float32(1.0), float32(1.0)

	// Função de ajuda local para buscar o nível de magma em qualquer offset
	getFluidLevel := func(dx, dy int32) float32 {
		neighborPos := tile.Position.Add(util.DFCoord{X: dx, Y: dy, Z: 0})
		if neighbor := tile.GetStore().GetTile(neighborPos); neighbor != nil {
			l := neighbor.MagmaLevel
			return float32(l) / 7.0
		}
		return float32(level) / 7.0
	}

	// Calculando a elevação nas 4 QUINAS
	nw := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(-1, 0) + getFluidLevel(-1, -1)) / 4.0
	ne := (getFluidLevel(0, 0) + getFluidLevel(0, -1) + getFluidLevel(1, 0) + getFluidLevel(1, -1)) / 4.0
	sw := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(-1, 0) + getFluidLevel(-1, 1)) / 4.0
	se := (getFluidLevel(0, 0) + getFluidLevel(0, 1) + getFluidLevel(1, 0) + getFluidLevel(1, 1)) / 4.0

	// Se o bloco de cima tiver magma, as quinas cravam no 1.0
	if upTile := tile.Up(); upTile != nil && upTile.MagmaLevel > 0 {
		nw, ne, sw, se = 1.0, 1.0, 1.0, 1.0
	}

	flowUV := [2]float32{0, 0} // Magma não tem vetor de fluxo externo

	// Face SUPERIOR (NW -> NE -> SE -> SW) - Ordem CLOCKWISE (CW)
	buffer.AddFaceUV(
		[3]float32{x, y + nw, z},         // NW
		[3]float32{x + w, y + ne, z},     // NE
		[3]float32{x + w, y + se, z - d}, // SE
		[3]float32{x, y + sw, z - d},     // SW
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, 1, 0}, color,
	)

	// Face INFERIOR plana
	buffer.AddFaceUV(
		[3]float32{x, y, z},
		[3]float32{x, y, z - d},
		[3]float32{x + w, y, z - d},
		[3]float32{x + w, y, z},
		flowUV, flowUV, flowUV, flowUV,
		[3]float32{0, -1, 0}, color,
	)

	// BORDAS/LATERAIS
	// Norte (+Z)
	if getFluidLevel(0, -1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x, y, z},
			[3]float32{x + w, y, z},
			[3]float32{x + w, y + ne, z},
			[3]float32{x, y + nw, z},
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, 1}, color,
		)
	}
	// Sul (-Z)
	if getFluidLevel(0, 1) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z - d},
			[3]float32{x, y, z - d},
			[3]float32{x, y + sw, z - d},
			[3]float32{x + w, y + se, z - d},
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{0, 0, -1}, color,
		)
	}
	// Oeste (-X)
	if getFluidLevel(-1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x, y, z - d},
			[3]float32{x, y, z},
			[3]float32{x, y + nw, z},
			[3]float32{x, y + sw, z - d},
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{-1, 0, 0}, color,
		)
	}
	// Leste (+X)
	if getFluidLevel(1, 0) < getFluidLevel(0, 0) {
		buffer.AddFaceUV(
			[3]float32{x + w, y, z},
			[3]float32{x + w, y, z - d},
			[3]float32{x + w, y + se, z - d},
			[3]float32{x + w, y + ne, z},
			flowUV, flowUV, flowUV, flowUV,
			[3]float32{1, 0, 0}, color,
		)
	}
}
