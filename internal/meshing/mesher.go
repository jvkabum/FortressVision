package meshing

import (
	"FortressVision/internal/mapdata"
	"FortressVision/internal/util"
)

// GeometryData contém os buffers de vértices para uma malha.
type GeometryData struct {
	Vertices []float32
	Normals  []float32
	Colors   []uint8
	UVs      []float32
	Indices  []uint16
}

// Clone cria uma cópia profunda dos dados para evitar corrupção de memória.
func (g GeometryData) Clone() GeometryData {
	clone := GeometryData{}
	if len(g.Vertices) > 0 {
		clone.Vertices = make([]float32, len(g.Vertices))
		copy(clone.Vertices, g.Vertices)
	}
	if len(g.Normals) > 0 {
		clone.Normals = make([]float32, len(g.Normals))
		copy(clone.Normals, g.Normals)
	}
	if len(g.Colors) > 0 {
		clone.Colors = make([]uint8, len(g.Colors))
		copy(clone.Colors, g.Colors)
	}
	if len(g.UVs) > 0 {
		clone.UVs = make([]float32, len(g.UVs))
		copy(clone.UVs, g.UVs)
	}
	if len(g.Indices) > 0 {
		clone.Indices = make([]uint16, len(g.Indices))
		copy(clone.Indices, g.Indices)
	}
	return clone
}

// Request representa um pedido de processamento de malha para um bloco.
type Request struct {
	Origin   util.DFCoord
	Data     *mapdata.MapDataStore
	MTime    int64 // Versão dos dados no momento da requisição
	FocusZ   int   // Camada Z que a câmera está focando
	MaxDepth int   // Quantas camadas abaixo verificar para oclusão
}

// Result contém os dados de geometria gerados para um bloco.
type Result struct {
	Origin   util.DFCoord
	Terreno  GeometryData
	Liquidos GeometryData
	MTime    int64 // Versão dos dados processados
}

// Mesher é a interface para geradores de malha.
type Mesher interface {
	Enqueue(req Request)
	Results() <-chan Result
	Stop()
}

// MeshBuffer auxilia na construção de malhas dinâmicas.
type MeshBuffer struct {
	Geometry GeometryData
}

// AddFace adiciona uma face retangular (quad) ao buffer.
func (b *MeshBuffer) AddFace(v1, v2, v3, v4 [3]float32, n [3]float32, c [4]uint8) {
	// Triângulo 1 (v1, v2, v3)
	b.addVertex(v1, n, c)
	b.addVertex(v2, n, c)
	b.addVertex(v3, n, c)

	// Triângulo 2 (v1, v3, v4)
	b.addVertex(v1, n, c)
	b.addVertex(v3, n, c)
	b.addVertex(v4, n, c)
}

func (b *MeshBuffer) addVertex(v [3]float32, n [3]float32, c [4]uint8) {
	b.Geometry.Vertices = append(b.Geometry.Vertices, v[0], v[1], v[2])
	b.Geometry.Normals = append(b.Geometry.Normals, n[0], n[1], n[2])
	b.Geometry.Colors = append(b.Geometry.Colors, c[0], c[1], c[2], c[3])
}

// MeshBuffer auxilia na construção de malhas dinâmicas.
// Agora atua apenas como um container, sem sistema de reset para evitar vazamento de memória Go -> C.
