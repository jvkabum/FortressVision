package mesher

import (
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
)

// MeshData encapsula os dados de geometria prontos para o GPU.
type MeshData struct {
	Vertices []rendering.Vertex
	Indices  []uint32
}

// SubMesh representa uma parte da geometria associada a um material específico.
type SubMesh struct {
	MaterialID int32
	Data       *MeshData
}

// ChunkMesh é o resultado final da transformação de um voxel chunk para geometria.
type ChunkMesh struct {
	Origin    matrix.Vec3
	SubMeshes map[int32]*MeshData
}

// NewMeshData cria uma nova estrutura de dados de malha vazia.
func NewMeshData() *MeshData {
	return &MeshData{
		Vertices: make([]rendering.Vertex, 0),
		Indices:  make([]uint32, 0),
	}
}

// AddQuad adiciona um quadrilátero à malha.
func (m *MeshData) AddQuad(v0, v1, v2, v3 rendering.Vertex) {
	startIdx := uint32(len(m.Vertices))
	m.Vertices = append(m.Vertices, v0, v1, v2, v3)
	
	// Primeiro Triângulo
	m.Indices = append(m.Indices, startIdx, startIdx+1, startIdx+2)
	// Segundo Triângulo
	m.Indices = append(m.Indices, startIdx, startIdx+2, startIdx+3)
}
