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

func (m *MeshData) AddTriangle(v0, v1, v2 rendering.Vertex) {
	startIdx := uint32(len(m.Vertices))
	m.Vertices = append(m.Vertices, v0, v1, v2)
	m.Indices = append(m.Indices, startIdx, startIdx+1, startIdx+2)
}

// AddGeometry junta outra de malha, aplicando um offset (posição), rotação opcionalmente na raiz e uma cor base.
func (m *MeshData) AddGeometry(verts []rendering.Vertex, indices []uint32, offset matrix.Vec3, color matrix.Color) {
	startIdx := uint32(len(m.Vertices))

	for _, v := range verts {
		// Adiciona o offset de posição
		v.Position = matrix.NewVec3(
			v.Position.X()+offset.X(),
			v.Position.Y()+offset.Y(),
			v.Position.Z()+offset.Z(),
		)

		// Aplica a sobreposição de cor mantendo o sombreamento original se existir no OBJ (multiplicação de cores simples)
		// Se o OBJ for branco, ele vai pegar puramente a color passada
		v.Color = matrix.NewColor(
			v.Color.R()*color.R(),
			v.Color.G()*color.G(),
			v.Color.B()*color.B(),
			v.Color.A()*color.A(),
		)

		m.Vertices = append(m.Vertices, v)
	}

	for _, idx := range indices {
		m.Indices = append(m.Indices, startIdx+idx)
	}
}
