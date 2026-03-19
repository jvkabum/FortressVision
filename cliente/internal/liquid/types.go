package liquid

// GeometryBuffer é a interface que qualquer buffer de geometria deve satisfazer
// para ser usado na geração de malhas de líquidos.
// O meshing.MeshBuffer implementa esta interface nativamente.
type GeometryBuffer interface {
	// AddFaceUV adiciona uma face (quad) ao buffer com coordenadas UV, normal e cor.
	AddFaceUV(v1, v2, v3, v4 [3]float32, uv1, uv2, uv3, uv4 [2]float32, n [3]float32, c [4]uint8)
}
