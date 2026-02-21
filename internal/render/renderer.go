package render

/*
#include <stdlib.h>
*/
import "C"

import (
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

    gl_Position = mvp * vec4(animatedPos, 1.0);
}
`

const waterFragmentShader = `
#version 330

in vec2 fragTexCoord;
in vec4 fragColor;
in vec3 fragNormal;

out vec4 finalColor;

void main()
{
    // Escala para as ondas
    vec2 uv = fragTexCoord * 2.5;
    
    // Múltiplas ondas senoidais cruzadas em diferentes frequências 
    // Isto gera o padrão de refração caústica na superfície
    float w1 = sin(uv.x + uv.y);
    float w2 = sin(uv.x * 0.7 - uv.y * 1.3);
    float w3 = cos(uv.x * 1.5 + uv.y * 0.8);
    
    // Média de ruído da superfície [-1.0 a 1.0]
    float wave = (w1 + w2 + w3) / 3.0; 
    
    // Converte a onda para um padrão afiado nas cristas (estilo espelho)
    wave = 1.0 - abs(wave);
    wave = pow(wave, 3.0); 
    
    vec4 waterColor = fragColor;
    
    // Deixa as cristas iluminadas com um leve Tonal Ciano e espuma brilhante
    waterColor.rgb += wave * vec3(0.15, 0.45, 0.65);
    
    // Adiciona escurecimento nas calhas (Vales)
    waterColor.rgb -= (1.0 - wave) * 0.1;
    
    // Mantemos intacto o canal Alpha vindo do CGO / CPU (Ex: 180 = 70% opaco)
    finalColor = vec4(waterColor.rgb, fragColor.a);
}
`

// BlockModel representa a geometria renderizável de um bloco do mapa.
type BlockModel struct {
	Origin      util.DFCoord
	Model       rl.Model
	LiquidModel rl.Model
	HasLiquid   bool
	Active      bool
	MTime       int64 // Versão dos dados (para cache)
}

// Renderer gerencia o upload e renderização de malhas na GPU.
type Renderer struct {
	mu         sync.RWMutex
	Models     map[util.DFCoord]*BlockModel
	MainShader rl.Shader

	WaterShader  rl.Shader
	WaterLocTime int32

	// Fila de modelos para purga (evita stutter)
	purgeQueue []util.DFCoord
}

// NewRenderer cria um novo renderizador.
func NewRenderer() *Renderer {
	r := &Renderer{
		Models:     make(map[util.DFCoord]*BlockModel),
		purgeQueue: make([]util.DFCoord, 0),
	}

	// Tenta carregar os Shaders Customizados
	if rl.IsWindowReady() {
		r.WaterShader = rl.LoadShaderFromMemory(waterVertexShader, waterFragmentShader)
		r.WaterLocTime = rl.GetShaderLocation(r.WaterShader, "time")
	}

	return r
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
			if old.HasLiquid {
				rl.UnloadModel(old.LiquidModel)
			}
		}
		delete(r.Models, res.Origin)
	}

	if len(res.Terreno.Vertices) == 0 && len(res.Liquidos.Vertices) == 0 {
		return
	}

	bm := &BlockModel{
		Origin: res.Origin,
		Active: true,
		MTime:  res.MTime,
	}

	if len(res.Terreno.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Terreno)
		rl.UploadMesh(&mesh, false)
		bm.Model = rl.LoadModelFromMesh(mesh)
	}

	if len(res.Liquidos.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Liquidos)
		rl.UploadMesh(&mesh, false)
		bm.LiquidModel = rl.LoadModelFromMesh(mesh)
		bm.HasLiquid = true

		// Associa o WaterShader ao material 0
		if r.WaterShader.ID != 0 && bm.LiquidModel.MaterialCount > 0 {
			materials := unsafe.Slice(bm.LiquidModel.Materials, bm.LiquidModel.MaterialCount)
			materials[0].Shader = r.WaterShader
		}
	}

	r.Models[res.Origin] = bm

	// Como a função geometryToMesh copiou as fatias para o C.malloc,
	// podemos limpar e devolver os Slices Go para o Pool reaproveitar sem afetar a GPU.
	meshing.PutMeshBuffer(&meshing.MeshBuffer{Geometry: res.Terreno})
	meshing.PutMeshBuffer(&meshing.MeshBuffer{Geometry: res.Liquidos})
}

func (r *Renderer) geometryToMesh(data meshing.GeometryData) rl.Mesh {
	mesh := rl.Mesh{}

	vCount := int32(len(data.Vertices) / 3)
	mesh.VertexCount = vCount
	mesh.TriangleCount = vCount / 3

	// IMPORTANTE para a estabilidade no Windows:
	// Alocamos memória no lado do C (C.malloc) para que o Raylib possa gerenciar
	// e liberar os buffers sem interferência do GC do Go. Isso elimina o crash 0xc0000374.

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
	if size == 0 {
		return nil
	}
	ptr := C.malloc(C.size_t(size))
	// Copiamos os dados de Go para C
	cSlice := unsafe.Slice((*byte)(ptr), size)
	goSlice := unsafe.Slice((*byte)(data), size)
	copy(cSlice, goSlice)
	return ptr
}

// Draw renderiza os blocos que estão dentro do raio de visão da câmera.
// Blocos no focusZ ignoram o culling de distância horizontal para manter o chão sempre visível.
func (r *Renderer) Draw(camPos rl.Vector3, focusZ int32) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Update da variavel global de tempo nos shaders
	timeVal := float32(rl.GetTime())
	if r.WaterShader.ID != 0 {
		rl.SetShaderValue(r.WaterShader, r.WaterLocTime, []float32{timeVal}, rl.ShaderUniformFloat)
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
		if bm.Model.MeshCount > 0 {
			rl.DrawModel(bm.Model, rl.Vector3{X: 0, Y: 0, Z: 0}, 1.0, rl.White)
		}
	}

	// ====== PASS 2: GEOMETRIA TRANSLÚCIDA (ÁGUA E MAGMA) ======
	// Somente após TODO o terreno sólido estar na tela, chamamos o Pass BlendAlpha
	// para que a água possa "mesclar" visualmente sua cor com a pedra no fundo.
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
