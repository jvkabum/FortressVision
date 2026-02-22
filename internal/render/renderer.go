package render

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"log"
	"sync"
	"unsafe"

	"FortressVision/internal/meshing"
	"FortressVision/internal/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

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
out float fragDist;

void main()
{
    // O TexCoord carrega a direção de fluxo: X (TexCoord.x), Y (TexCoord.y)
    vec2 flowDir = vertexTexCoord;

    // Deslocamento animado pelo tempo
    float speed = 2.0;
    vec2 offset = flowDir * time * speed;
    
    // O UV final baseia-se na coord do mundo + fluxo
    fragTexCoord = vertexPosition.xz + offset;
    
    fragColor = vertexColor;
    fragNormal = vertexNormal;

    vec3 animatedPos = vertexPosition;
    // Pequena ondulação se escoando
    if (length(flowDir) > 0.1) {
    	animatedPos.y += sin(time * 3.0 + vertexPosition.x) * 0.05 * length(flowDir);
	}

    // Distância para o Fog (no espaço da view)
    vec4 viewPos = mvp * vec4(animatedPos, 1.0);
    fragDist = length(viewPos.xyz);

    gl_Position = viewPos;
}
`

const waterFragmentShader = `
#version 330

in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;
in float fragDist;

out vec4 finalColor;

void main()
{
    // Escala para as ondas
    vec2 uv = fragTexCoord * 2.5;
    
    // Múltiplas ondas senoidais cruzadas
    float w1 = sin(uv.x + uv.y);
    float w2 = sin(uv.x * 0.7 - uv.y * 1.3);
    float w3 = cos(uv.x * 1.5 + uv.y * 0.8);
    
    float wave = (w1 + w2 + w3) / 3.0; 
    
    // Brilho muito mais intenso nas cristas (Specular faux)
    float specular = pow(wave, 12.0) * 0.6;
    
    vec4 waterColor = fragColor;
    
    waterColor.rgb += wave * vec3(0.1, 0.5, 0.8);
    waterColor.rgb += specular;
    waterColor.rgb -= (1.0 - wave) * 0.05;
    
    // FOG CALCULATION
    vec3 fogColor = vec3(0.12, 0.12, 0.16); // Cor escura do céu/vazio
    float fogDensity = 0.005;
    float fogFactor = exp(-pow(fragDist * fogDensity, 2.0));
    fogFactor = clamp(fogFactor, 0.0, 1.0);

    vec3 finalRGB = mix(fogColor, waterColor.rgb, fogFactor);
    finalColor = vec4(finalRGB, fragColor.a * 0.8 * fogFactor);
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

    // Adiciona uma leve variação de cor baseada no ruído (Grit/Splatting visual)
    float n = hash(floor(fragTexCoord * 10.0));
    vec4 mixedColor = texelColor;
    mixedColor.rgb *= (0.9 + 0.2 * n);

    // Efeito de Neve (Baseado na Normal e Uniforme)
    // Se a normal aponta para cima (Y > 0.7) e snowAmount > 0
    float snowFactor = clamp(fragNormal.y, 0.0, 1.0);
    snowFactor = pow(snowFactor, 4.0) * snowAmount;
    
    // Adiciona variação de ruído à neve para não ficar plana
    float snowNoise = hash(fragTexCoord * 5.0 + vec2(time * 0.01));
    snowFactor *= (0.8 + 0.4 * snowNoise);
    
    mixedColor.rgb = mix(mixedColor.rgb, vec3(0.9, 0.95, 1.0), snowFactor);

    finalColor = mixedColor * fragColor;
}
`

// BlockModel representa a geometria renderizável de um bloco do mapa.
type BlockModel struct {
	Origin      util.DFCoord
	Model       rl.Model            // Geometria padrão (sem textura)
	MatModels   map[string]rl.Model // Modelos separados por textura (stone, grass, etc)
	LiquidModel rl.Model
	HasLiquid   bool
	Active      bool
	MTime       int64                   // Versão dos dados (para cache)
	Instances   []meshing.ModelInstance // Instâncias de modelos 3D neste bloco
}

// Renderer gerencia o upload e renderização de malhas na GPU.
type Renderer struct {
	mu     sync.RWMutex
	Models map[util.DFCoord]*BlockModel
	// Shaders
	WaterShader   rl.Shader
	PlantShader   rl.Shader
	TerrainShader rl.Shader

	// Uniforms
	timeLoc        int32
	waterTimeLoc   int32
	plantTimeLoc   int32
	terrainTimeLoc int32
	snowAmountLoc  int32

	// Texturas Premium
	Textures map[string]rl.Texture2D

	// Modelos 3D carregados (shrub, tree, etc)
	Models3D map[string]rl.Model

	// Sistema de Clima (Fase 8)
	Weather *ParticleSystem

	// Fila de modelos para purga (evita stutter)
	purgeQueue []util.DFCoord
}

// NewRenderer cria um novo renderizador.
func NewRenderer() *Renderer {
	r := &Renderer{
		Models:     make(map[util.DFCoord]*BlockModel),
		purgeQueue: make([]util.DFCoord, 0),
		Textures:   make(map[string]rl.Texture2D),
		Models3D:   make(map[string]rl.Model),
	}

	// Tenta carregar os Shaders Customizados
	if rl.IsWindowReady() {
		r.TerrainShader = rl.LoadShaderFromMemory(terrainVertexShader, terrainFragmentShader)
		r.PlantShader = rl.LoadShaderFromMemory(plantVertexShader, terrainFragmentShader) // Reusa o fragment shader
		r.WaterShader = rl.LoadShaderFromMemory(waterVertexShader, waterFragmentShader)

		r.terrainTimeLoc = rl.GetShaderLocation(r.TerrainShader, "time")
		r.snowAmountLoc = rl.GetShaderLocation(r.TerrainShader, "snowAmount")
		r.plantTimeLoc = rl.GetShaderLocation(r.PlantShader, "time")
		r.waterTimeLoc = rl.GetShaderLocation(r.WaterShader, "time")

		// Carregar Texturas Premium
		r.loadTextures()

		// Carregar Modelos 3D
		r.loadModels()
	}

	r.Weather = NewParticleSystem(2000)

	return r
}

func (r *Renderer) loadTextures() {
	assets := []string{"stone", "grass", "wood", "marble", "ore", "gem", "plant"}
	for _, name := range assets {
		path := fmt.Sprintf("assets/textures/%s.png", name)
		tex := rl.LoadTexture(path)
		if tex.ID != 0 {
			rl.GenTextureMipmaps(&tex)
			rl.SetTextureFilter(tex, rl.FilterTrilinear)
			rl.SetTextureWrap(tex, rl.WrapRepeat)
			r.Textures[name] = tex
			log.Printf("[Renderer] Textura carregada: %s", path)
		} else {
			log.Printf("[Renderer] FALHA ao carregar textura: %s", path)
		}
	}
}

func (r *Renderer) loadModels() {
	type modelEntry struct {
		name string
		path string
	}

	entries := []modelEntry{
		{"shrub", "assets/models/shrub.glb"},
		{"tree_trunk", "assets/models/TREE.obj"},
		{"tree_branches", "assets/models/shrub.glb"},
		{"mushroom", "assets/models/SAPLING.obj"},
	}

	for _, entry := range entries {
		log.Printf("[Renderer] CARGA: Tentando '%s' de '%s'...", entry.name, entry.path)

		model := rl.LoadModel(entry.path)
		if model.MeshCount > 0 {
			r.Models3D[entry.name] = model
			log.Printf("[Renderer] SUCESSO: '%s' carregado", entry.name)
		} else {
			log.Printf("[Renderer] AVISO: '%s' falhou (sem meshes)", entry.name)
		}
	}
}

// HasModel verifica se já existe um modelo carregado para esta coordenada e com a mesma versão.
func (r *Renderer) GetModelVersion(coord util.DFCoord) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if bm, ok := r.Models[coord]; ok {
		return bm.MTime
	}
	return -1
}

// UploadResult converte um resultado de meshing em um modelo Raylib GPU.
// Deve ser chamado na thread principal (onde o contexto OpenGL é válido).
func (r *Renderer) UploadResult(res meshing.Result) {
	// PROTEÇÃO: Não processar se o contexto Raylib não estiver pronto
	if !rl.IsWindowReady() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Se já existe um modelo nesta posição, libera o antigo (Raylib agora pode dar free com segurança)
	if old, ok := r.Models[res.Origin]; ok {
		if old.Active {
			rl.UnloadModel(old.Model)
			for _, m := range old.MatModels {
				rl.UnloadModel(m)
			}
			if old.HasLiquid {
				rl.UnloadModel(old.LiquidModel)
			}
		}
		delete(r.Models, res.Origin)
	}

	if len(res.Terreno.Vertices) == 0 && len(res.Liquidos.Vertices) == 0 && len(res.MaterialGeometries) == 0 {
		return
	}

	bm := &BlockModel{
		Origin:    res.Origin,
		Active:    true,
		MTime:     res.MTime,
		MatModels: make(map[string]rl.Model),
	}

	// 1. Upload de geometria sem textura (Legado/Fallback)
	if len(res.Terreno.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Terreno)
		rl.UploadMesh(&mesh, false)
		r.freeMeshRAM(&mesh) // Libera RAM após upload para VRAM
		bm.Model = rl.LoadModelFromMesh(mesh)
		if bm.Model.MaterialCount > 0 {
			materials := unsafe.Slice(bm.Model.Materials, bm.Model.MaterialCount)
			materials[0].Shader = r.TerrainShader
		}
	}

	// 2. Upload de geometria por material (Com textura)
	for matName, geo := range res.MaterialGeometries {
		if len(geo.Vertices) > 0 {
			mesh := r.geometryToMesh(geo)
			rl.UploadMesh(&mesh, false)
			r.freeMeshRAM(&mesh) // Libera RAM após upload para VRAM
			model := rl.LoadModelFromMesh(mesh)

			// Aplicar shader SEMPRE
			if model.MaterialCount > 0 {
				materials := unsafe.Slice(model.Materials, model.MaterialCount)
				materials[0].Shader = r.TerrainShader
				// Aplicar textura se existir
				if tex, ok := r.Textures[matName]; ok {
					rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
				}
			}
			bm.MatModels[matName] = model
		}
	}

	// 3. Upload de líquidos
	if len(res.Liquidos.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Liquidos)
		rl.UploadMesh(&mesh, false)
		r.freeMeshRAM(&mesh) // Libera RAM após upload para VRAM
		bm.LiquidModel = rl.LoadModelFromMesh(mesh)
		bm.HasLiquid = true

		// Associa o WaterShader ao material 0
		if r.WaterShader.ID != 0 && bm.LiquidModel.MaterialCount > 0 {
			materials := unsafe.Slice(bm.LiquidModel.Materials, bm.LiquidModel.MaterialCount)
			materials[0].Shader = r.WaterShader
		}
	}

	r.Models[res.Origin] = bm

	// 4. Salvar instâncias de modelos 3D (arbustos, etc)
	bm.Instances = res.ModelInstances

	// Como a função geometryToMesh copiou as fatias para o C.malloc,
	// podemos limpar e devolver os Slices Go para o Pool reaproveitar sem afetar a GPU.
	meshing.PutMeshBuffer(&meshing.MeshBuffer{Geometry: res.Terreno})
	meshing.PutMeshBuffer(&meshing.MeshBuffer{Geometry: res.Liquidos})
}

func (r *Renderer) geometryToMesh(data meshing.GeometryData) rl.Mesh {
	var mesh rl.Mesh

	vCount := int32(len(data.Vertices) / 3)
	mesh.VertexCount = vCount
	mesh.TriangleCount = vCount / 3

	// Limpamos/Inicializamos ponteiros Explicitamente para evitar Garbage no CGO
	mesh.Vertices = nil
	mesh.Normals = nil
	mesh.Colors = nil
	mesh.Texcoords = nil
	mesh.Indices = nil
	mesh.AnimVertices = nil
	mesh.AnimNormals = nil
	mesh.Tangents = nil

	if len(data.Vertices) > 0 {
		mesh.Vertices = (*float32)(r.copyToC(unsafe.Pointer(&data.Vertices[0]), len(data.Vertices)*4))
	}
	if len(data.Normals) > 0 {
		mesh.Normals = (*float32)(r.copyToC(unsafe.Pointer(&data.Normals[0]), len(data.Normals)*4))
	}
	if len(data.Colors) > 0 {
		mesh.Colors = (*uint8)(r.copyToC(unsafe.Pointer(&data.Colors[0]), len(data.Colors)))
	}
	if len(data.UVs) > 0 {
		mesh.Texcoords = (*float32)(r.copyToC(unsafe.Pointer(&data.UVs[0]), len(data.UVs)*4))
	}

	return mesh
}

// copyToC aloca memória C e copia os dados.
func (r *Renderer) copyToC(data unsafe.Pointer, size int) unsafe.Pointer {
	if size <= 0 || data == nil {
		return nil
	}
	// Usamos MemAlloc da Raylib para garantir compatibilidade de heap se possível,
	// mas como raylib-go não expõe, usamos C.malloc e limpamos MANUALMENTE.
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		log.Printf("[CGO] FALHA crítica de alocação de memória (%d bytes)", size)
		return nil
	}
	// Copiamos os dados de Go para C
	cSlice := unsafe.Slice((*byte)(ptr), size)
	goSlice := unsafe.Slice((*byte)(data), size)
	copy(cSlice, goSlice)
	return ptr
}

// freeMeshRAM libera a memória principal (C) associada a uma malha após o upload para a GPU.
func (r *Renderer) freeMeshRAM(mesh *rl.Mesh) {
	if mesh.Vertices != nil {
		C.free(unsafe.Pointer(mesh.Vertices))
		mesh.Vertices = nil
	}
	if mesh.Normals != nil {
		C.free(unsafe.Pointer(mesh.Normals))
		mesh.Normals = nil
	}
	if mesh.Colors != nil {
		C.free(unsafe.Pointer(mesh.Colors))
		mesh.Colors = nil
	}
	if mesh.Texcoords != nil {
		C.free(unsafe.Pointer(mesh.Texcoords))
		mesh.Texcoords = nil
	}
}

// Draw renderiza os blocos que estão dentro do raio de visão da câmera.
// Blocos no focusZ ignoram o culling de distância horizontal para manter o chão sempre visível.
func (r *Renderer) Draw(camPos rl.Vector3, focusZ int32) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Update da variavel global de tempo nos shaders
	timeVal := float32(rl.GetTime())
	if r.WaterShader.ID != 0 {
		rl.SetShaderValue(r.WaterShader, r.waterTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
	}
	if r.PlantShader.ID != 0 {
		rl.SetShaderValue(r.PlantShader, r.plantTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
	}
	if r.TerrainShader.ID != 0 {
		rl.SetShaderValue(r.TerrainShader, r.terrainTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
		// Snow Amount (Poderia vir do world state, mas vamos fixar para teste por enquanto)
		rl.SetShaderValue(r.TerrainShader, r.snowAmountLoc, []float32{0.8}, rl.ShaderUniformFloat)
	}

	// Raio padrão para níveis verticais distantes
	const cullingRadiusSq = 80 * 80

	// DISTANCE CULLING: REMOVIDO para Visão Ilimitada
	// Renderiza tudo o que estiver carregado na GPU

	// ====== PASS 1: GEOMETRIA OPACA (TERRENO, PAREDES) ======
	// Importante para garantir que o Z-Buffer oclua objetos corretamente antes da água por cima.
	for _, bm := range r.Models {
		if !bm.Active {
			continue
		}
		// Desenha modelo padrão
		if bm.Model.MeshCount > 0 {
			rl.DrawModel(bm.Model, rl.Vector3{X: 0, Y: 0, Z: 0}, 1.0, rl.White)
		}
		// Desenha modelos texturizados
		for _, m := range bm.MatModels {
			if m.MeshCount > 0 {
				rl.DrawModel(m, rl.Vector3{X: 0, Y: 0, Z: 0}, 1.0, rl.White)
			}
		}
		// Desenha instâncias de modelos 3D (arbustos, árvores, etc)
		for _, inst := range bm.Instances {
			if model3d, ok := r.Models3D[inst.ModelName]; ok {
				pos := rl.Vector3{X: inst.Position[0], Y: inst.Position[1], Z: inst.Position[2]}
				tintColor := rl.NewColor(inst.Color[0], inst.Color[1], inst.Color[2], 255)

				// Aplica a textura e o shader ao material
				if model3d.MaterialCount > 0 {
					materials := unsafe.Slice(model3d.Materials, model3d.MaterialCount)

					// Escolhe o shader correto: PlantShader para vegetação, TerrainShader para rampas/outros
					shader := r.TerrainShader
					if inst.ModelName == "shrub" || inst.ModelName == "tree_trunk" || inst.ModelName == "tree_branches" || inst.ModelName == "mushroom" {
						shader = r.PlantShader
					}

					materials[0].Shader = shader

					if inst.TextureName != "" {
						if tex, ok := r.Textures[inst.TextureName]; ok {
							rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
						}
					}
				}

				// O DrawModelEx aceita uma rotação (eixo Y), escala e cor (tint).
				rl.DrawModelEx(model3d, pos, rl.Vector3{X: 0, Y: 1, Z: 0}, inst.Rotation, rl.Vector3{X: inst.Scale, Y: inst.Scale, Z: inst.Scale}, tintColor)
			}
		}
	}

	// ====== PASS 2: GEOMETRIA TRANSLÚCIDA (ÁGUA E MAGMA) ======
	// Somente após TODO o terreno sólido estar na tela, chamamos o Pass BlendAlpha
	// para que a água possa "mesclar" visualmente sua cor com a pedra no fundo.
	// ====== PASS 2: GEOMETRIA TRANSLÚCIDA (ÁGUA E MAGMA) ======
	rl.BeginBlendMode(rl.BlendAlpha)
	for _, bm := range r.Models {
		if !bm.Active {
			continue
		}
		if bm.HasLiquid {
			rl.DrawModel(bm.LiquidModel, rl.Vector3{X: 0, Y: 0, Z: 0}, 1.0, rl.White)
		}
	}
	rl.EndBlendMode()

	// ====== PASS 3: EFEITOS CLIMÁTICOS (NEVE/CHUVA) ======
	if r.Weather != nil {
		r.Weather.Update(rl.GetFrameTime(), camPos)
		r.Weather.Draw()
	}
}

// Purge desativado para Unlimited Vision
func (r *Renderer) Purge(center util.DFCoord, radius float32) {
	// r.mu.Lock()
	// defer r.mu.Unlock()
	// ... purga removida ...
}

// ProcessPurge executa a remoção física de modelos da GPU de forma limitada.
func (r *Renderer) ProcessPurge() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove no máximo 2 modelos por frame para evitar travadas
	limit := 2
	if len(r.purgeQueue) < limit {
		limit = len(r.purgeQueue)
	}

	for i := 0; i < limit; i++ {
		coord := r.purgeQueue[0]
		r.purgeQueue = r.purgeQueue[1:]

		if bm, ok := r.Models[coord]; ok {
			rl.UnloadModel(bm.Model)
			if bm.HasLiquid {
				rl.UnloadModel(bm.LiquidModel)
			}
			delete(r.Models, coord)
		}
	}
}

// Unload libera todos os recursos de GPU.
func (r *Renderer) Unload() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, bm := range r.Models {
		rl.UnloadModel(bm.Model)
		if bm.HasLiquid {
			rl.UnloadModel(bm.LiquidModel)
		}
	}
	r.Models = make(map[util.DFCoord]*BlockModel)
}
