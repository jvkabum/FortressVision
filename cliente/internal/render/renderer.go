package render

/*
#include <stdlib.h>
*/
import "C"

import (
	"log"
	"math"
	"sync"
	"unsafe"

	"FortressVision/cliente/internal/assets"
	"FortressVision/cliente/internal/meshing"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Shaders e struct BlockModel movidos para arquivos separados (shaders.go e block_model.go)

type Renderer struct {
	mu     sync.RWMutex
	Models map[util.DFCoord]*BlockModel

	// Shaders e Uniforms
	TerrainShader          rl.Shader
	TerrainInstancedShader rl.Shader
	PlantShader            rl.Shader
	PlantInstancedShader   rl.Shader
	WaterShader            rl.Shader

	timeLoc        int32
	waterTimeLoc   int32
	waterCamPosLoc int32
	plantTimeLoc   int32
	terrainTimeLoc int32
	snowAmountLoc  int32

	// Texturas Premium
	Textures map[string]rl.Texture2D

	// Modelos 3D carregados (shrub, tree, etc)
	Models3D map[string]rl.Model

	// Gerenciador de Assets (JSON config)
	AssetMgr *assets.Manager

	// Sistema de Clima (Fase 8)
	Weather *ParticleSystem

	// Fila de modelos para purga (evita stutter)
	purgeQueue []util.DFCoord

	PropMgr *PropManager // Sistema de GPU Instancing (Fase 33)

	debugInstCount int // DEBUG: contador frame para log temporário
}

// NewRenderer cria um novo renderizador.
func NewRenderer() *Renderer {
	r := &Renderer{
		Models:     make(map[util.DFCoord]*BlockModel),
		purgeQueue: make([]util.DFCoord, 0),
		Textures:   make(map[string]rl.Texture2D),
		Models3D:   make(map[string]rl.Model),
	}

	// Inicializar o Gerenciador de Assets (JSON)
	mgr, err := assets.NewManager("assets/config")
	if err != nil {
		log.Printf("[Renderer] AVISO: Asset Manager não inicializado: %v", err)
	} else {
		r.AssetMgr = mgr
		log.Printf("[Renderer] Asset Manager carregado com sucesso")
	}

	// Tenta carregar os Shaders Customizados
	if rl.IsWindowReady() {
		r.TerrainShader = rl.LoadShaderFromMemory(terrainVertexShader, terrainFragmentShader)
		r.TerrainInstancedShader = rl.LoadShaderFromMemory(terrainInstancedVertexShader, terrainFragmentShader)
		r.PlantShader = rl.LoadShaderFromMemory(plantVertexShader, plantFragmentShader)
		r.PlantInstancedShader = rl.LoadShaderFromMemory(plantInstancedVertexShader, plantFragmentShader)
		r.WaterShader = rl.LoadShaderFromMemory(waterVertexShader, waterFragmentShader)

		// Registrar localizações de uniforms padrão para que Raylib preencha automaticamente
		// Locs é um ponteiro bruto (*int32) que aponta para um array em C (32 floats)
		locsT := unsafe.Slice(r.TerrainShader.Locs, 32)
		locsT[0] = rl.GetShaderLocation(r.TerrainShader, "texture0")    // SHADER_LOC_MAP_DIFFUSE
		locsT[12] = rl.GetShaderLocation(r.TerrainShader, "colDiffuse") // SHADER_LOC_COLOR_DIFFUSE

		locsTI := unsafe.Slice(r.TerrainInstancedShader.Locs, 32)
		locsTI[0] = rl.GetShaderLocation(r.TerrainInstancedShader, "texture0")
		locsTI[12] = rl.GetShaderLocation(r.TerrainInstancedShader, "colDiffuse")

		locsP := unsafe.Slice(r.PlantShader.Locs, 32)
		locsP[0] = rl.GetShaderLocation(r.PlantShader, "texture0")
		locsP[12] = rl.GetShaderLocation(r.PlantShader, "colDiffuse")

		locsPI := unsafe.Slice(r.PlantInstancedShader.Locs, 32)
		locsPI[0] = rl.GetShaderLocation(r.PlantInstancedShader, "texture0")
		locsPI[12] = rl.GetShaderLocation(r.PlantInstancedShader, "colDiffuse")

		r.terrainTimeLoc = rl.GetShaderLocation(r.TerrainShader, "time")
		r.snowAmountLoc = rl.GetShaderLocation(r.TerrainShader, "snowAmount")
		r.plantTimeLoc = rl.GetShaderLocation(r.PlantShader, "time")
		r.waterTimeLoc = rl.GetShaderLocation(r.WaterShader, "time")
		r.waterCamPosLoc = rl.GetShaderLocation(r.WaterShader, "camPos")

		// Carregar Texturas Premium
		r.loadTextures()

		// Carregar Modelos 3D (via JSON config)
		r.loadModels()
	}

	log.Printf("[DEBUG INIT] NewRenderer() finalizado. Models3D=%d, Textures=%d", len(r.Models3D), len(r.Textures))

	r.Weather = NewParticleSystem(2000)

	r.PropMgr = NewPropManager()

	return r
}

// Métodos de carga de assets movidos para assets_loader.go

// GetModelVersion verifica se já existe um modelo carregado para esta coordenada e com a mesma versão.
func (r *Renderer) GetModelVersion(coord util.DFCoord) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if bm, ok := r.Models[coord]; ok {
		return bm.MTime
	}
	return -1
}

// UploadResult converte um resultado de meshing em um modelo Raylib GPU.
func (r *Renderer) UploadResult(res meshing.Result) {
	if !rl.IsWindowReady() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

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

	if len(res.Terreno.Vertices) == 0 && len(res.Liquidos.Vertices) == 0 && len(res.MaterialGeometries) == 0 && len(res.ModelInstances) == 0 {
		return
	}

	bm := &BlockModel{
		Origin:    res.Origin,
		Active:    true,
		MTime:     res.MTime,
		MatModels: make(map[string]rl.Model),
		Instances: res.ModelInstances,
	}

	if len(res.ModelInstances) > 0 {
		log.Printf("[Renderer] Recebidas %d instâncias para o chunk %s (Primeira rampa: %v)",
			len(res.ModelInstances), res.Origin.String(), res.ModelInstances[0].IsRamp)
	}

	if len(res.Terreno.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Terreno)
		rl.UploadMesh(&mesh, false)
		// r.freeMeshRAM(&mesh) // DESATIVADO para permitir Raycasting (Fase 35 Fix)
		bm.Model = rl.LoadModelFromMesh(mesh)
		if bm.Model.MaterialCount > 0 {
			materials := unsafe.Slice(bm.Model.Materials, bm.Model.MaterialCount)
			materials[0].Shader = r.TerrainShader
		}
	}

	for matName, geo := range res.MaterialGeometries {
		if len(geo.Vertices) > 0 {
			mesh := r.geometryToMesh(geo)
			rl.UploadMesh(&mesh, false)
			// r.freeMeshRAM(&mesh) // DESATIVADO para permitir Raycasting (Fase 35 Fix)
			model := rl.LoadModelFromMesh(mesh)
			if model.MaterialCount > 0 {
				materials := unsafe.Slice(model.Materials, model.MaterialCount)
				materials[0].Shader = r.TerrainShader
				if tex, ok := r.Textures[matName]; ok {
					rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
				}
			}
			bm.MatModels[matName] = model
		}
	}

	if len(res.Liquidos.Vertices) > 0 {
		mesh := r.geometryToMesh(res.Liquidos)
		rl.UploadMesh(&mesh, false)
		r.freeMeshRAM(&mesh)
		bm.LiquidModel = rl.LoadModelFromMesh(mesh)
		bm.HasLiquid = true
		if r.WaterShader.ID != 0 && bm.LiquidModel.MaterialCount > 0 {
			materials := unsafe.Slice(bm.LiquidModel.Materials, bm.LiquidModel.MaterialCount)
			materials[0].Shader = r.WaterShader
		}
	}

	r.Models[res.Origin] = bm
}

func (r *Renderer) geometryToMesh(data meshing.GeometryData) rl.Mesh {
	var mesh rl.Mesh
	vCount := int32(len(data.Vertices) / 3)
	mesh.VertexCount = vCount
	mesh.TriangleCount = vCount / 3

	mesh.Vertices = nil
	mesh.Normals = nil
	mesh.Colors = nil
	mesh.Texcoords = nil

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

func (r *Renderer) copyToC(data unsafe.Pointer, size int) unsafe.Pointer {
	if size <= 0 || data == nil {
		return nil
	}
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		return nil
	}
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
func (r *Renderer) Draw(camera3d rl.Camera3D, focusZ int32) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	camPos := camera3d.Position

	if r.debugInstCount == 0 {
		log.Printf("[DEBUG DRAW] Primeiro Draw(). Models3D=%d, Models=%d, r=%p, focusZ=%d, camPos=(%.1f,%.1f,%.1f)",
			len(r.Models3D), len(r.Models), r, focusZ, camPos.X, camPos.Y, camPos.Z)
	}

	// Update da variavel global de tempo nos shaders
	timeVal := float32(rl.GetTime())
	if r.WaterShader.ID != 0 {
		rl.SetShaderValue(r.WaterShader, r.waterTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
		rl.SetShaderValue(r.WaterShader, r.waterCamPosLoc, []float32{camPos.X, camPos.Y, camPos.Z}, rl.ShaderUniformVec3)
	}
	if r.PlantShader.ID != 0 {
		rl.SetShaderValue(r.PlantShader, r.plantTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
	}
	if r.TerrainShader.ID != 0 {
		rl.SetShaderValue(r.TerrainShader, r.terrainTimeLoc, []float32{timeVal}, rl.ShaderUniformFloat)
		// Snow Amount (Fase 28: Depende do clima ativo)
		rl.SetShaderValue(r.TerrainShader, r.snowAmountLoc, []float32{r.Weather.GetSnowAccumulation()}, rl.ShaderUniformFloat)
	}

	// Raio de visão generoso (120 unidades = ~120 tiles de distância)
	// Isso evita o efeito de "neblina preta" mas protege a CPU de milhares de draw calls inúteis.
	const viewRadiusSq = 120.0 * 120.0

	// PASS 1: TERRENO
	for _, bm := range r.Models {
		if !bm.Active {
			continue
		}
		// Culling Vertical de Terreno: Relaxado (+/- 128 níveis) para ver buracos profundos
		diffZ := util.Abs(bm.Origin.Z - focusZ)
		if diffZ > 128 {
			continue
		}

		// DEBUG: Ignorar culling de distância por enquanto
		// distSq := util.DistSq(camPos, rl.Vector3{X: float32(bm.Origin.X), Y: camPos.Y, Z: float32(-bm.Origin.Y)})
		// if bm.Origin.Z != focusZ && distSq > viewRadiusSq {
		// 	continue
		// }

		if bm.Model.MeshCount > 0 {
			rl.DrawModel(bm.Model, rl.Vector3{0, 0, 0}, 1.0, rl.White)
		}
		for _, m := range bm.MatModels {
			if m.MeshCount > 0 {
				rl.DrawModel(m, rl.Vector3{0, 0, 0}, 1.0, rl.White)
			}
		}
	}

	// PASS 1.5: VEGETACAO E RAMPAS (GPU INSTANCING - Ultra Performance)
	r.PropMgr.Clear()
	const instViewRadiusSq = 90.0 * 90.0

	for _, bm := range r.Models {
		if !bm.Active || len(bm.Instances) == 0 {
			continue
		}

		// Culling Vertical de Props/Rampas: Relaxado (+/- 64 níveis)
		diffZ := util.Abs(bm.Origin.Z - focusZ)
		if diffZ > 64 {
			continue
		}

		// Culling de chunk rápido
		// distSq := util.DistSq(camPos, rl.Vector3{X: float32(bm.Origin.X), Y: camPos.Y, Z: float32(-bm.Origin.Y)})
		// if bm.Origin.Z != focusZ && distSq > viewRadiusSq {
		// 	continue
		// }

		for _, inst := range bm.Instances {
			pos := rl.Vector3{X: inst.Position[0], Y: inst.Position[1], Z: inst.Position[2]}
			if util.DistSq(camPos, pos) > instViewRadiusSq {
				continue
			}

			model3d, ok := r.Models3D[inst.ModelName]
			if !ok || model3d.MeshCount == 0 {
				continue
			}

			// Preparamos o material e shader para este tipo de prop (INSTANCED)
			material := unsafe.Slice(model3d.Materials, model3d.MaterialCount)[0]
			shader := r.TerrainInstancedShader
			if isPlantModel(inst.ModelName) && !inst.IsRamp {
				shader = r.PlantInstancedShader
			}
			material.Shader = shader

			// Aplica textura
			if inst.TextureName != "" {
				if tex, ok := r.Textures[inst.TextureName]; ok {
					rl.SetMaterialTexture(&material, rl.MapDiffuse, tex)
				}
			}

			// Adiciona ao lote de instanciamento global
			r.PropMgr.AddInstance(inst, unsafe.Slice(model3d.Meshes, model3d.MeshCount)[0], material)
		}
	}

	// Desenha tudo em lotes otimizados (1 draw call por tipo)
	r.PropMgr.DrawAll(camera3d)

	// PASS 2: LIQUIDOS
	rl.BeginBlendMode(rl.BlendAlpha)
	for _, bm := range r.Models {
		if !bm.Active || !bm.HasLiquid {
			continue
		}
		rl.DrawModel(bm.LiquidModel, rl.Vector3{0, 0, 0}, 1.0, rl.White)
	}
	rl.EndBlendMode()

	// PASS 3: CLIMA
	if r.Weather != nil {
		r.Weather.Update(rl.GetFrameTime(), camPos)
		r.Weather.Draw()
	}
}

func (r *Renderer) Purge(center util.DFCoord, radius float32) {}

func (r *Renderer) ProcessPurge() {
	r.mu.Lock()
	defer r.mu.Unlock()
	limit := 2
	if len(r.purgeQueue) < limit {
		limit = len(r.purgeQueue)
	}
	for i := 0; i < limit; i++ {
		coord := r.purgeQueue[0]
		r.purgeQueue = r.purgeQueue[1:]
		if bm, ok := r.Models[coord]; ok {
			rl.UnloadModel(bm.Model)
			for _, m := range bm.MatModels {
				rl.UnloadModel(m)
			}
			if bm.HasLiquid {
				rl.UnloadModel(bm.LiquidModel)
			}
			delete(r.Models, coord)
		}
	}
}

func (r *Renderer) Unload() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, bm := range r.Models {
		rl.UnloadModel(bm.Model)
		for _, m := range bm.MatModels {
			rl.UnloadModel(m)
		}
		if bm.HasLiquid {
			rl.UnloadModel(bm.LiquidModel)
		}
	}
	r.Models = make(map[util.DFCoord]*BlockModel)
}

// GetRayCollision verifica qual bloco do terreno foi atingido pelo raio do mouse.
func (r *Renderer) GetRayCollision(ray rl.Ray) (util.DFCoord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	log.Printf("[Renderer] GetRayCollision iniciado. Testando contra %d modelos de chunks", len(r.Models))
	var closestDist float32 = 1000000.0
	var hit bool
	var hitPos rl.Vector3

	for _, bm := range r.Models {
		if !bm.Active {
			continue
		}

		// Testamos contra o modelo principal de terreno (várias meshes)
		if bm.Model.MeshCount > 0 {
			meshes := unsafe.Slice(bm.Model.Meshes, bm.Model.MeshCount)
			for i := int32(0); i < bm.Model.MeshCount; i++ {
				collision := rl.GetRayCollisionMesh(ray, meshes[i], bm.Model.Transform)
				if collision.Hit && collision.Distance < closestDist {
					closestDist = collision.Distance
					hitPos = collision.Point
					hit = true
				}
			}
		}

		// Testamos contra os modelos de materiais
		for _, m := range bm.MatModels {
			if m.MeshCount > 0 {
				meshes := unsafe.Slice(m.Meshes, m.MeshCount)
				for i := int32(0); i < m.MeshCount; i++ {
					collision := rl.GetRayCollisionMesh(ray, meshes[i], m.Transform)
					if collision.Hit && collision.Distance < closestDist {
						closestDist = collision.Distance
						hitPos = collision.Point
						hit = true
					}
				}
			}
		}

		// NOVO: Testamos contra Props e Rampas (Instâncias)
		for _, inst := range bm.Instances {
			// Pegamos o modelo correspondente (Rampa ou Prop)
			model, ok := r.Models3D[inst.ModelName]
			if !ok {
				continue
			}

			// Geramos a matriz de transformação da mesma forma que o PropManager
			// (Nota: Isso garante que o teste de colisão seja feito na posição exata onde o modelo é desenhado)
			scaleMat := rl.MatrixScale(inst.Scale, inst.Scale, inst.Scale)
			rotMat := rl.MatrixRotateY(inst.Rotation * (math.Pi / 180.0))
			transMat := rl.MatrixTranslate(inst.Position[0], inst.Position[1], inst.Position[2])
			matrix := rl.MatrixMultiply(rl.MatrixMultiply(transMat, rotMat), scaleMat)

			if model.MeshCount > 0 {
				meshes := unsafe.Slice(model.Meshes, model.MeshCount)
				for i := int32(0); i < model.MeshCount; i++ {
					// Testamos contra cada mesh do modelo 3D usando a matriz da instância específica
					collision := rl.GetRayCollisionMesh(ray, meshes[i], matrix)
					if collision.Hit && collision.Distance < closestDist {
						closestDist = collision.Distance
						hitPos = collision.Point
						hit = true
					}
				}
			}
		}
	}

	if hit {
		dir := rl.Vector3Normalize(ray.Direction)
		hitPos.X += dir.X * 0.01
		hitPos.Y += dir.Y * 0.01
		hitPos.Z += dir.Z * 0.01

		return util.WorldToDFCoord(hitPos), true
	}

	return util.DFCoord{}, false
}

// DrawSelection desenha um cubo de destaque no bloco selecionado.
func (r *Renderer) DrawSelection(coord util.DFCoord) {
	pos := util.DFToWorldCenter(coord)
	// Ajustamos para o centro vertical do bloco (DF Z + 0.5)
	pos.Y += 0.5
	rl.DrawCubeWires(pos, 1.01, 1.01, 1.01, rl.Yellow)
}

func isPlantModel(modelName string) bool {
	return modelName == "shrub" || modelName == "tree_body" || modelName == "tree_trunk" ||
		modelName == "tree_branches" || modelName == "tree_twigs" || modelName == "branches" ||
		modelName == "mushroom" || modelName == "sapling" || (len(modelName) > 12 && modelName[:12] == "tree_branch_")
}
