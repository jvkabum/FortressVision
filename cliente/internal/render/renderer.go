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
	"FortressVision/cliente/internal/liquid"
	"FortressVision/cliente/internal/meshing"
	"FortressVision/shared/util"
	"FortressVision/cliente/internal/comp"

	"github.com/mlange-42/ark/ecs"
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
	ModelShader            rl.Shader

	timeLoc        int32
	waterTimeLoc   int32
	waterCamPosLoc int32
	plantTimeLoc   int32
	terrainTimeLoc int32
	snowAmountLoc  int32
	modelMatLoc    int32

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

	// --- ECS (Ark) ---
	World       ecs.World
	queryPlants ecs.Filter4[comp.Position, comp.Renderable, comp.Rotation, comp.Scale]
	// Mappers (otimizados para criação de entidades)
	vegMapper *ecs.Map5[comp.Position, comp.Rotation, comp.Scale, comp.Renderable, comp.ChunkInfo]
}

// NewRenderer cria um novo renderizador.
func NewRenderer() *Renderer {
	r := &Renderer{
		Models:     make(map[util.DFCoord]*BlockModel),
		purgeQueue: make([]util.DFCoord, 0),
		Textures:   make(map[string]rl.Texture2D),
		Models3D:   make(map[string]rl.Model),
		World:      *ecs.NewWorld(),
	}

	// Inicializar Helpers de Query Genéricos
	r.queryPlants = *ecs.NewFilter4[comp.Position, comp.Renderable, comp.Rotation, comp.Scale](&r.World)
	
	// Inicializar Mappers
	r.vegMapper = ecs.NewMap5[comp.Position, comp.Rotation, comp.Scale, comp.Renderable, comp.ChunkInfo](&r.World)

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

		r.terrainTimeLoc = rl.GetShaderLocation(r.TerrainShader, "time")
		r.snowAmountLoc = rl.GetShaderLocation(r.TerrainShader, "snowAmount")

		// Carregar shader dedicado para modelos (Rampas, Escadas)
		r.ModelShader = rl.LoadShaderFromMemory(modelVertexShader, terrainFragmentShader)
		r.modelMatLoc = rl.GetShaderLocation(r.ModelShader, "matModel")

		r.plantTimeLoc = rl.GetShaderLocation(r.PlantShader, "time")
		r.waterTimeLoc = rl.GetShaderLocation(r.WaterShader, "time")
		r.waterCamPosLoc = rl.GetShaderLocation(r.WaterShader, "camPos")

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

		liquid.TraceDebug("WaterShader carregado com sucesso (verificar ID no console se disponivel)")

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

	// --- INTEGRAÇÃO ECS (Ark) ---
	// 1. Limpar entidades antigas deste chunk
	r.cleanupChunkEntities(res.Origin)

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
		Instances: res.ModelInstances, // Keep for now, but ECS will take over rendering
	}

	// 3. Criar entidades no ECS para cada instância enviada
	for _, inst := range res.ModelInstances {
		r.vegMapper.NewEntity(
			&comp.Position{X: inst.Position[0], Y: inst.Position[1], Z: inst.Position[2]},
			&comp.Rotation{Angle: inst.Rotation},
			&comp.Scale{Value: inst.Scale},
			&comp.Renderable{
				ModelName:   inst.ModelName,
				TextureName: inst.TextureName,
				Color:       inst.Color,
				IsRamp:      inst.IsRamp,
			},
			&comp.ChunkInfo{X: res.Origin.X, Y: res.Origin.Y, Z: res.Origin.Z},
		)
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
		// r.freeMeshRAM(&mesh) // DESATIVADO para teste (estilo terreno)
		bm.LiquidModel = rl.LoadModelFromMesh(mesh)
		bm.HasLiquid = true
		liquid.TraceUpload(res.Origin.X, res.Origin.Y, res.Origin.Z, len(res.Liquidos.Vertices)/3)
		if r.WaterShader.ID != 0 && bm.LiquidModel.MaterialCount > 0 {
			materials := unsafe.Slice(bm.LiquidModel.Materials, bm.LiquidModel.MaterialCount)
			materials[0].Shader = r.WaterShader
		}
	}

	r.Models[res.Origin] = bm
}

func (r *Renderer) cleanupChunkEntities(origin util.DFCoord) {
	// Filtro para encontrar todas as entidades que pertencem a este chunk
	filter := ecs.NewFilter1[comp.ChunkInfo](&r.World)
	query := filter.Query()
	
	var toRemove []ecs.Entity
	for query.Next() {
		cinfo := query.Get()
		if cinfo.X == origin.X && cinfo.Y == origin.Y && cinfo.Z == origin.Z {
			toRemove = append(toRemove, query.Entity())
		}
	}
	query.Close()

	for _, e := range toRemove {
		r.World.RemoveEntity(e)
	}
}

func (r *Renderer) geometryToMesh(data meshing.GeometryData) rl.Mesh {
	var mesh rl.Mesh

	// Se a malha usa Indices (EBO), o contagem de triângulos baseia-se nos índices
	// Caso contrário (malhas unindexed), baseia-se nos vértices
	vCount := int32(len(data.Vertices) / 3)
	mesh.VertexCount = vCount
	if len(data.Indices) > 0 {
		mesh.TriangleCount = int32(len(data.Indices) / 3)
	} else {
		mesh.TriangleCount = vCount / 3
	}

	mesh.Vertices = nil
	mesh.Normals = nil
	mesh.Colors = nil
	mesh.Texcoords = nil
	mesh.Indices = nil

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
	if len(data.Indices) > 0 {
		mesh.Indices = (*uint16)(r.copyToC(unsafe.Pointer(&data.Indices[0]), len(data.Indices)*2))
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
	if mesh.Indices != nil {
		C.free(unsafe.Pointer(mesh.Indices))
		mesh.Indices = nil
	}
}

// Draw renderiza os blocos que estão dentro do raio de visão da câmera.
// Blocos no focusZ ignoram o culling de distância horizontal para manter o chão sempre visível.
func (r *Renderer) Draw(camera3d rl.Camera3D, focusZ int32) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Desabilitar backface culling: No sistema de níveis, faces devem ser visíveis de todos os ângulos.
	// O winding (0,2,1)+(0,3,2) faz algumas faces apontarem "para dentro", e o culling as descartaria
	// quando vistas "por trás". Desabilitando, garantimos visibilidade total.
	rl.DisableBackfaceCulling()

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
		// Z-Slicing (Estilo Timberborn): Ocultar tudo o que está acima do nível focado
		if bm.Origin.Z > focusZ {
			continue
		}

		// Culling Vertical de Terreno: Relaxado para ver buracos profundos
		diffZ := util.Abs(bm.Origin.Z - focusZ)
		if diffZ > 64 {
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

	// PASS 1.5: VEGETACAO E RAMPAS (ECS - Ark + GPU INSTANCING)
	r.PropMgr.Clear()
	const instViewRadiusSq = 150.0 * 150.0

	// Usamos o Query do ECS para encontrar tudo que precisa ser desenhado
	query := r.queryPlants.Query()
	for query.Next() {
		posComp, rendComp, rotComp, scaleComp := query.Get()

		// Culling de distância
		pos := rl.Vector3{X: posComp.X, Y: posComp.Y, Z: posComp.Z}
		if util.DistSq(camPos, pos) > instViewRadiusSq {
			continue
		}

		// Z-Slicing (Baseado na posição Y do mundo, que é o Z do DF)
		if int32(posComp.Y) > focusZ {
			continue
		}

		model3d, ok := r.Models3D[rendComp.ModelName]
		if !ok {
			continue
		}
		if model3d.MeshCount == 0 {
			continue
		}

		// Preparamos o material e shader (Simplificado por enquanto como no código original)
		material := unsafe.Slice(model3d.Materials, model3d.MaterialCount)[0]
		shader := r.TerrainInstancedShader
		if isPlantModel(rendComp.ModelName) && !rendComp.IsRamp && r.PlantInstancedShader.ID != 0 {
			shader = r.PlantInstancedShader
		}
		material.Shader = shader

		if rendComp.TextureName != "" {
			if tex, ok := r.Textures[rendComp.TextureName]; ok {
				rl.SetMaterialTexture(&material, rl.MapDiffuse, tex)
			}
		}

		// Se for rampa, desenhamos direto para teste de visibilidade (Fase 45)
		if rendComp.IsRamp {
			// Usar ModelShader dedicado para rampas (Fase 51)
			materials := unsafe.Slice(model3d.Materials, model3d.MaterialCount)
			if rendComp.TextureName != "" {
				if tex, ok := r.Textures[rendComp.TextureName]; ok {
					rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
				}
			}
			materials[0].Shader = r.ModelShader
			
			// Escala Gold Standard (0.5/0.3/0.5)
			c := rl.Color{R: rendComp.Color[0], G: rendComp.Color[1], B: rendComp.Color[2], A: rendComp.Color[3]}
			rl.DrawModelEx(model3d, pos, rl.Vector3{X: 0, Y: 1, Z: 0}, rotComp.Angle, rl.Vector3{X: scaleComp.Value, Y: scaleComp.Value, Z: scaleComp.Value}, c)
			continue
		}

		// Adiciona cada mesh do modelo à fila de instancing
		meshes := unsafe.Slice(model3d.Meshes, model3d.MeshCount)
		for i := 0; i < int(model3d.MeshCount); i++ {
			meshMaterial := material
			if int32(i) < model3d.MaterialCount {
				meshMaterial = unsafe.Slice(model3d.Materials, model3d.MaterialCount)[i]
				meshMaterial.Shader = material.Shader
			}
			
			r.PropMgr.AddInstanceFromECS(posComp, rendComp, rotComp.Angle, scaleComp.Value, meshes[i], meshMaterial, i)
		}
	}
	query.Close()

	// Desenha tudo em lotes otimizados (1 draw call por tipo)
	r.PropMgr.DrawAll(camera3d)

	// PASS 2: LIQUIDOS
	rl.BeginBlendMode(rl.BlendAlpha)
	// Enviar uniformes globais para a água uma vez por frame
	if r.WaterShader.ID != 0 {
		rl.SetShaderValue(r.WaterShader, r.waterTimeLoc, []float32{float32(rl.GetTime())}, rl.ShaderUniformFloat)
		rl.SetShaderValue(r.WaterShader, r.waterCamPosLoc, []float32{camera3d.Position.X, camera3d.Position.Y, camera3d.Position.Z}, rl.ShaderUniformVec3)
	}
	for origin, bm := range r.Models {
		if !bm.Active || !bm.HasLiquid {
			continue
		}

		// Z-Slicing para Líquidos
		if origin.Z > focusZ {
			continue
		}
		liquid.TraceDraw(origin.X, origin.Y, origin.Z)
		rl.DrawModel(bm.LiquidModel, rl.Vector3{X: 0, Y: 0, Z: 0}, 1.0, rl.White)
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
