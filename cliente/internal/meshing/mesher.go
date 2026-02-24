package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"sync"
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

// ModelInstance representa uma instância de um modelo 3D no mundo.
type ModelInstance struct {
	ModelName   string     // Nome do modelo (ex: "shrub")
	TextureName string     // Nome da textura (ex: "stone", "grass")
	Position    [3]float32 // Posição no mundo
	Scale       float32    // Escala
	Rotation    float32    // Rotação em graus (eixo Y)
	Color       [4]uint8   // Cor/tint
}

// Result contém os dados de geometria gerados para um bloco.
type Result struct {
	Origin             util.DFCoord
	Terreno            GeometryData // Legado (se não for usar textura)
	Liquidos           GeometryData
	MaterialGeometries map[string]GeometryData // Nova geometria separada por nome de textura
	ModelInstances     []ModelInstance         // Instâncias de modelos 3D (arbustos, árvores, etc)
	MTime              int64                   // Versão dos dados processados
}

// Mesher é a interface para geradores de malha.
type Mesher interface {
	Enqueue(req Request)
	Results() <-chan Result
	Stop()
}

// Global Poll para reciclar MeshBuffers e evitar alocação excessiva (GC Pressure)
var meshBufferPool = sync.Pool{
	New: func() interface{} {
		return &MeshBuffer{
			Geometry: GeometryData{
				Vertices: make([]float32, 0, 4096),
				Normals:  make([]float32, 0, 4096),
				Colors:   make([]uint8, 0, 4096),
			},
		}
	},
}

// GetMeshBuffer aloca ou recicla um buffer vazio para meshing.
func GetMeshBuffer() *MeshBuffer {
	return meshBufferPool.Get().(*MeshBuffer)
}

// PutMeshBuffer zera os ponteiros e devolve a memória para o Pool.
func PutMeshBuffer(b *MeshBuffer) {
	if b == nil {
		return
	}
	b.Geometry.Vertices = b.Geometry.Vertices[:0]
	b.Geometry.Normals = b.Geometry.Normals[:0]
	b.Geometry.Colors = b.Geometry.Colors[:0]
	b.Geometry.UVs = b.Geometry.UVs[:0]
	b.Geometry.Indices = b.Geometry.Indices[:0]
	meshBufferPool.Put(b)
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
	// Default UV 0,0 for standard vertices
	b.Geometry.UVs = append(b.Geometry.UVs, 0, 0)
}

// AddFaceUV adiciona uma face ao buffer suportando coordenadas de textura UV / custom variables.
func (b *MeshBuffer) AddFaceUV(v1, v2, v3, v4 [3]float32, uv1, uv2, uv3, uv4 [2]float32, n [3]float32, c [4]uint8) {
	// Triângulo 1 (v1, v2, v3)
	b.addVertexUV(v1, uv1, n, c)
	b.addVertexUV(v2, uv2, n, c)
	b.addVertexUV(v3, uv3, n, c)

	// Triângulo 2 (v1, v3, v4)
	b.addVertexUV(v1, uv1, n, c)
	b.addVertexUV(v3, uv3, n, c)
	b.addVertexUV(v4, uv4, n, c)
}

func (b *MeshBuffer) addVertexUV(v [3]float32, uv [2]float32, n [3]float32, c [4]uint8) {
	b.Geometry.Vertices = append(b.Geometry.Vertices, v[0], v[1], v[2])
	b.Geometry.Normals = append(b.Geometry.Normals, n[0], n[1], n[2])
	b.Geometry.Colors = append(b.Geometry.Colors, c[0], c[1], c[2], c[3])
	b.Geometry.UVs = append(b.Geometry.UVs, uv[0], uv[1])
}

// AddTriangle adiciona uma face triangular ao buffer.
func (b *MeshBuffer) AddTriangle(v1, v2, v3 [3]float32, n [3]float32, c [4]uint8) {
	b.addVertex(v1, n, c)
	b.addVertex(v2, n, c)
	b.addVertex(v3, n, c)
}

// AddTriangleUV adiciona uma face triangular ao buffer com coordenadas UV.
func (b *MeshBuffer) AddTriangleUV(v1, v2, v3 [3]float32, uv1, uv2, uv3 [2]float32, normal [3]float32, color [4]uint8) {
	b.addVertexUV(v1, uv1, normal, color)
	b.addVertexUV(v2, uv2, normal, color)
	b.addVertexUV(v3, uv3, normal, color)
}

func (b *MeshBuffer) AddFaceAOStandard(v1 [3]float32, c1 [4]uint8, v2 [3]float32, c2 [4]uint8, v3 [3]float32, c3 [4]uint8, v4 [3]float32, c4 [4]uint8, normal [3]float32) {
	b.AddTriangle(v1, v2, v3, normal, c1)
	b.AddTriangle(v1, v3, v4, normal, c1)
}

func (b *MeshBuffer) AddFaceUVStandard(v1 [3]float32, uv1 [2]float32, c1 [4]uint8, v2 [3]float32, uv2 [2]float32, c2 [4]uint8, v3 [3]float32, uv3 [2]float32, c3 [4]uint8, v4 [3]float32, uv4 [2]float32, c4 [4]uint8, normal [3]float32) {
	b.AddTriangleUV(v1, v2, v3, uv1, uv2, uv3, normal, c1)
	b.addVertexUV(v1, uv1, normal, c1) // Re-adicionando v1 para o segundo triângulo
	b.addVertexUV(v3, uv3, normal, c3)
	b.addVertexUV(v4, uv4, normal, c4)
}

// MeshBuffer auxilia na construção de malhas dinâmicas.
// Agora atua apenas como um container, sem sistema de reset para evitar vazamento de memória Go -> C.
