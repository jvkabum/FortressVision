package render

import (
	"FortressVision/cliente/internal/mesher"
	"fmt"
	"kaijuengine.com/engine"
	"kaijuengine.com/engine/cameras"
	"kaijuengine.com/engine/collision"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
	"kaijuengine.com/registry/shader_data_registry"
)

// CameraViewCuller adapta a interface cameras.Camera para rendering.ViewCuller
type CameraViewCuller struct {
	Camera cameras.Camera
}

func (c *CameraViewCuller) IsInView(box collision.AABB) bool {
	return box.IntersectsFrustum(c.Camera.Frustum())
}

func (c *CameraViewCuller) ViewChanged() bool {
	return c.Camera.IsDirty()
}

// DrawFoundation cria o plano verde inicial e retorna seu transform para controle dinâmico
func CreateFoundation(host *engine.Host, center matrix.Vec3) *matrix.Transform {
	// Vértices do plano de solo (200x200 metros para ser bem visível)
	verts := []rendering.Vertex{
		{Position: matrix.NewVec3(-100, 0, -100), Color: matrix.ColorGreen(), Normal: matrix.Vec3Up()},
		{Position: matrix.NewVec3( 100, 0, -100), Color: matrix.ColorGreen(), Normal: matrix.Vec3Up()},
		{Position: matrix.NewVec3( 100, 0,  100), Color: matrix.ColorGreen(), Normal: matrix.Vec3Up()},
		{Position: matrix.NewVec3(-100, 0,  100), Color: matrix.ColorGreen(), Normal: matrix.Vec3Up()},
	}
	indices := []uint32{0, 2, 1, 0, 3, 2}
	
	mesh := host.MeshCache().Mesh("gizmo_foundation", verts, indices)
	mat, _ := host.MaterialCache().Material("basic.material")
	sd := shader_data_registry.Create("basic")
	if sd != nil {
		sd.(*shader_data_registry.ShaderDataStandard).Color = matrix.ColorWhite()
	}

	trans := &matrix.Transform{}
	trans.SetupRawTransform()
	trans.SetPosition(center)

	draw := rendering.Drawing{
		Mesh:       mesh,
		Material:   mat,
		ShaderData: sd,
		Transform:  trans,
		ViewCuller: &CameraViewCuller{Camera: host.PrimaryCamera()},
	}
	host.Drawings.AddDrawing(draw)
	return trans
}

// CreateRedBall cria uma esfera vermelha para diagnóstico e retorna seu transform
func CreateRedBall(host *engine.Host, pos matrix.Vec3) *matrix.Transform {
	mesh := rendering.NewMeshSphere(host.MeshCache(), 1.0, 32, 32)
	mat, _ := host.MaterialCache().Material("basic.material")
	sd := shader_data_registry.Create("basic")
	if sd != nil {
		sd.(*shader_data_registry.ShaderDataStandard).Color = matrix.NewColor(1, 0, 0, 1)
	}

	trans := &matrix.Transform{}
	trans.SetupRawTransform()
	trans.SetPosition(pos)

	draw := rendering.Drawing{
		Mesh:       mesh,
		Material:   mat,
		ShaderData: sd,
		Transform:  trans,
		ViewCuller: &CameraViewCuller{Camera: host.PrimaryCamera()},
	}
	host.Drawings.AddDrawing(draw)
	return trans
}

// CreateChunkDrawings cria as estruturas de desenho de um chunk preparadas para submissão por frame.
func CreateChunkDrawings(host *engine.Host, cm *mesher.ChunkMesh, meshKey string) []rendering.Drawing {
	if cm == nil {
		return nil
	}

	mat, _ := host.MaterialCache().Material("basic.material")
	viewCuller := &CameraViewCuller{Camera: host.PrimaryCamera()}
	var draws []rendering.Drawing

	for matID, data := range cm.SubMeshes {
		if len(data.Vertices) == 0 {
			continue
		}

		// Chave única para o cache de malha (ID do Chunk + Material + Modificador de Versão)
		key := fmt.Sprintf("%s_%d", meshKey, matID)
		mesh := host.MeshCache().Mesh(key, data.Vertices, data.Indices)
		
		sd := shader_data_registry.Create("basic")
		if sd != nil {
			sd.(*shader_data_registry.ShaderDataStandard).Color = matrix.ColorWhite()
		}

		trans := &matrix.Transform{}
		trans.SetupRawTransform()
		trans.SetPosition(cm.Origin)

		drawings := rendering.Drawing{
			Mesh:       mesh,
			Material:   mat,
			ShaderData: sd,
			Transform:  trans,
			ViewCuller: viewCuller,
		}
		draws = append(draws, drawings)
	}
	return draws
}

// DrawChunks renderiza toda a geometria gerada pelo mesher (Legacy - Evitar usar em loop)
func DrawChunks(host *engine.Host, meshingMgr *mesher.Manager) {
	meshes := meshingMgr.GetMeshes()
	for _, cm := range meshes {
		key := fmt.Sprintf("legacy_%v", cm.Origin)
		draws := CreateChunkDrawings(host, cm, key)
		for _, d := range draws {
			host.Drawings.AddDrawing(d)
		}
	}
}
