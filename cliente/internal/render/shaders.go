package render

const waterVertexShader = `
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

// Gerstner Wave: retorna deslocamento (XYZ) e normal parcial
vec3 gerstnerWave(vec2 pos, vec2 dir, float steepness, float wavelength, float t) {
    float k = 6.2831853 / wavelength;
    float c = sqrt(9.8 / k);
    float a = steepness / k;
    float phase = k * (dot(dir, pos) - c * t);
    return vec3(
        dir.x * a * cos(phase),
        a * sin(phase),
        dir.y * a * cos(phase)
    );
}

void main()
{
    vec2 flowDir = vertexTexCoord;
    float speed = 2.0;
    vec2 offset = flowDir * time * speed;
    fragTexCoord = vertexPosition.xz + offset;
    fragColor = vertexColor;

    vec3 pos = vertexPosition;

    // 3 camadas de ondas Gerstner com direções diferentes
    float flowMag = length(flowDir);
    float waveIntensity = max(flowMag, 0.3); // Mesmo água parada tem leve ondulação

    pos += gerstnerWave(pos.xz, normalize(vec2(1.0, 0.6)),  0.15 * waveIntensity, 3.0, time) * 0.6;
    pos += gerstnerWave(pos.xz, normalize(vec2(-0.4, 1.0)), 0.10 * waveIntensity, 2.0, time * 1.3) * 0.4;
    pos += gerstnerWave(pos.xz, normalize(vec2(0.7, -0.5)), 0.08 * waveIntensity, 1.5, time * 0.8) * 0.3;

    // Recalcular normal aproximada via derivadas das ondas
    float eps = 0.1;
    vec3 pX = pos + gerstnerWave(pos.xz + vec2(eps, 0.0), normalize(vec2(1.0, 0.6)), 0.15, 3.0, time) * 0.1;
    vec3 pZ = pos + gerstnerWave(pos.xz + vec2(0.0, eps), normalize(vec2(1.0, 0.6)), 0.15, 3.0, time) * 0.1;
    vec3 tangentX = vec3(eps, pX.y - pos.y, 0.0);
    vec3 tangentZ = vec3(0.0, pZ.y - pos.y, eps);
    fragNormal = normalize(cross(tangentZ, tangentX));

    fragWorldPos = pos;
    gl_Position = mvp * vec4(pos, 1.0);
}
`

const waterFragmentShader = `
#version 330

in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;
in vec3 fragWorldPos;

uniform float time;
uniform vec3 camPos;

out vec4 finalColor;

// Hash para ruído procedural
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
    // ===== CORES BASE =====
    vec3 shallowColor = vec3(0.15, 0.65, 0.65); // Turquesa claro
    vec3 deepColor    = vec3(0.02, 0.12, 0.30); // Azul escuro profundo

    // A profundidade está codificada no alpha do vértice (0.0 = raso, 1.0 = profundo)
    float depth = fragColor.a;
    vec3 baseColor = mix(shallowColor, deepColor, depth * 0.7);

    // ===== ONDAS MULTI-CAMADA =====
    vec2 uv = fragTexCoord;
    float w1 = sin(uv.x * 3.0 + uv.y * 2.0 + time * 1.5) * 0.5 + 0.5;
    float w2 = sin(uv.x * 1.7 - uv.y * 2.3 + time * 1.1) * 0.5 + 0.5;
    float w3 = sin(uv.x * 4.0 + uv.y * 1.5 + time * 2.0) * 0.5 + 0.5;
    float wave = (w1 + w2 + w3) / 3.0;

    // Variação de cor baseada nas ondas
    baseColor += wave * vec3(0.05, 0.15, 0.20);
    baseColor -= (1.0 - wave) * vec3(0.02, 0.05, 0.08);

    // ===== ESPUMA NAS BORDAS =====
    // Espuma aparece em água mais rasa (depth baixo) e nas cristas das ondas
    float foamNoise = noise(uv * 8.0 + time * 0.5);
    float foamEdge = smoothstep(0.0, 0.4, 1.0 - depth); // Mais forte em água rasa
    float foamCrest = smoothstep(0.65, 0.85, wave);       // Cristas das ondas
    float foam = max(foamEdge, foamCrest) * foamNoise;
    foam = smoothstep(0.3, 0.7, foam);
    baseColor = mix(baseColor, vec3(0.85, 0.92, 0.95), foam * 0.6);

    // ===== CAÚSTICAS =====
    vec2 caustUV = fragWorldPos.xz * 1.5;
    float c1 = sin(caustUV.x * 3.0 + time + sin(caustUV.y * 2.0 + time * 0.7));
    float c2 = sin(caustUV.y * 3.5 - time * 0.8 + cos(caustUV.x * 2.5 + time));
    float caustic = pow(abs(c1 * c2), 2.0) * 0.15;
    baseColor += caustic * vec3(0.3, 0.6, 0.8);

    // ===== FRESNEL (Reflexão/Transparência por ângulo) =====
    vec3 viewDir = normalize(camPos - fragWorldPos);
    vec3 normal = normalize(fragNormal);
    float fresnel = pow(1.0 - max(dot(viewDir, normal), 0.0), 3.0);
    fresnel = clamp(fresnel, 0.0, 1.0);

    // Reflexão fake do "céu" (gradiente claro)
    vec3 skyReflection = vec3(0.45, 0.65, 0.85);
    baseColor = mix(baseColor, skyReflection, fresnel * 0.4);

    // ===== SPECULAR (Blinn-Phong) =====
    vec3 lightDir = normalize(vec3(0.5, 0.8, 0.3)); // Direção do "sol"
    vec3 halfVec = normalize(lightDir + viewDir);
    float spec = pow(max(dot(normal, halfVec), 0.0), 64.0);
    baseColor += spec * vec3(1.0, 0.95, 0.8) * 0.5;

    // ===== FOG =====
    float dist = length(camPos - fragWorldPos);
    vec3 fogColor = vec3(0.12, 0.12, 0.16);
    float fogDensity = 0.005;
    float fogFactor = exp(-pow(dist * fogDensity, 2.0));
    fogFactor = clamp(fogFactor, 0.0, 1.0);

    vec3 finalRGB = mix(fogColor, baseColor, fogFactor);

    // Transparência: mais opaco no fundo, mais transparente nas bordas rasas
    float alpha = mix(0.55, 0.85, depth) * fogFactor;
    alpha = max(alpha, foam * 0.7); // Espuma é mais opaca

    finalColor = vec4(finalRGB, alpha);
}
`

const plantVertexShader = `
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
out float fragHeight;

void main() {
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    fragNormal = vertexNormal;
    fragHeight = vertexPosition.y;
    
    vec3 pos = vertexPosition;
    // Animação de vento: balanço horizontal baseado na altura (Y)
    float windStrength = 0.15;
    float freq = 2.0;
    float move = sin(time * freq + pos.x * 0.5 + pos.z * 0.5) * windStrength * pos.y;
    pos.x += move;
    pos.z += move * 0.3;

    gl_Position = mvp * vec4(pos, 1.0);
}
`

const plantFragmentShader = `
#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;
in float fragHeight;

uniform sampler2D texture0;
uniform vec4 colDiffuse; // Raylib passa le tint/color aqui
uniform float time;

out vec4 finalColor;

void main() {
    vec4 texelColor = texture(texture0, fragTexCoord);
    if (texelColor.a < 0.2) discard; 

    // Iluminação básica
    vec3 lightDir = normalize(vec3(0.5, 1.0, 0.3));
    float diff = max(dot(fragNormal, lightDir), 0.0);
    vec3 ambient = vec3(0.4, 0.4, 0.4);
    vec3 light = ambient + vec3(0.6) * diff;

    // Combina textura, cor do vértice (AO) e tint (colDiffuse)
    vec4 color = texelColor * fragColor * colDiffuse;
    color.rgb *= light;

    // Leve variação de cor baseada na altura
    color.rgb *= (0.8 + 0.2 * smoothstep(0.0, 1.0, fragHeight));

    finalColor = color;
}
`

const terrainVertexShader = `
#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec3 vertexNormal;
in vec4 vertexColor;

uniform mat4 mvp;

out vec2 fragTexCoord;
out vec4 fragColor;
out vec3 fragNormal;
out float fragHeight;

void main() {
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    fragNormal = vertexNormal;
    fragHeight = vertexPosition.y;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}
`

const terrainFragmentShader = `
#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;
in float fragHeight;

uniform sampler2D texture0;
uniform vec4 colDiffuse;
uniform float time;
uniform float snowAmount; // 0.0 a 1.0

out vec4 finalColor;

// Função de ruído simples para "Ground Splatting" visual
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

void main() {
    vec4 texelColor = texture(texture0, fragTexCoord);
    if (texelColor.a < 0.1) discard; 

    // Variação de cor baseada no ruído
    float n = hash(floor(fragTexCoord * 10.0));
    vec4 mixedColor = texelColor * fragColor * colDiffuse;
    mixedColor.rgb *= (0.9 + 0.2 * n);

    // Efeito de Neve
    float snowFactor = clamp(fragNormal.y, 0.0, 1.0);
    snowFactor = pow(snowFactor, 4.0) * snowAmount;
    float snowNoise = hash(fragTexCoord * 5.0 + vec2(time * 0.01));
    snowFactor *= (0.8 + 0.4 * snowNoise);
    
    mixedColor.rgb = mix(mixedColor.rgb, vec3(0.9, 0.95, 1.0), snowFactor);

    finalColor = mixedColor;
}
`
