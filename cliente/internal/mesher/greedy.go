package mesher

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
)

// materialKey identifica o par completo (tipo + índice), evitando colisões
// entre materiais que compartilham o mesmo MatIndex em categorias diferentes.
func materialKey(pair dfproto.MatPair) int32 {
	const indexBits = 20
	return ((pair.MatType + 1) << indexBits) | (pair.MatIndex + 1)
}

// getTileColor usa as cores reais recebidas do DFHack. O fallback cinza é
// usado apenas durante a janela em que a lista de materiais ainda não chegou.
func getTileColor(tile *mapdata.Tile) matrix.Color {
	if tile == nil || tile.GetStore() == nil || tile.GetStore().MatStore == nil {
		return matrix.NewColor(0.5, 0.5, 0.5, 1.0)
	}

	c := tile.GetStore().MatStore.GetTileColor(tile)
	return matrix.NewColor(
		matrix.Float(c.R)/255.0,
		matrix.Float(c.G)/255.0,
		matrix.Float(c.B)/255.0,
		matrix.Float(c.A)/255.0,
	)
}

// GenerateChunkMesh transforma dados de voxels em geometria 3D otimizada.
func GenerateChunkMesh(chunk *mapdata.Chunk) *ChunkMesh {
	origin := util.DFToWorldPos(chunk.Origin)

	cm := &ChunkMesh{
		Origin:    matrix.NewVec3(matrix.Float(origin.X), matrix.Float(origin.Y), matrix.Float(origin.Z)),
		SubMeshes: make(map[int32]*MeshData),
	}

	// SubMesh[0]: Malha Sólida Principal (Paredes, Pisos, Escadas, Rampas)
	mainMesh := NewMeshData()
	cm.SubMeshes[0] = mainMesh

	// SubMesh[1]: Malha de Líquidos Translúcidos (Água, Magma)
	liquidMesh := NewMeshData()
	cm.SubMeshes[1] = liquidMesh

	// 1 e 2. Mesh 2D Unificada Extremamente Otimizada (W e H estendidos com Oclusão)
	meshGreedy2D(chunk, mainMesh)

	// 3. Mesh Especiais (Escadas, Rampas)
	meshSpecials(chunk, mainMesh)

	// 4. Mesh de Líquidos
	meshLiquids(chunk, liquidMesh)

	return cm
}

func shouldDrawFace(chunk *mapdata.Chunk, x, y int, dx, dy int, isFloor bool) bool {
	nx, ny := x+dx, y+dy
	if nx < 0 || nx >= 16 || ny < 0 || ny >= 16 {
		return true // Borda do chunk, sempre renderiza nessa etapa planar
	}
	neighbor := chunk.Tiles[nx][ny]
	if neighbor == nil || neighbor.Hidden {
		return true
	}
	shape := neighbor.Shape()
	if shape == dfproto.ShapeNoShape {
		return true
	}

	// Oclusão Euclidiana Absoluta:
	// Elimina polígonos que encostam fisicamente em outros formando maciço visível.
	if !isFloor {
		// Para paredes: elas ocultam com outras paredes e blocos cheios
		if shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification || shape == dfproto.ShapeTreeShape {
			return false
		}
	} else {
		// Para pisos (DF Floors): laterais invisíveis se encontram vizinhos (que sustentam os floors ou são os mesmos).
		if shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification || shape == dfproto.ShapeFloor || shape == dfproto.ShapeTreeShape || shape == dfproto.ShapeBoulder {
			return false
		}
	}
	return true
}

func meshGreedy2D(chunk *mapdata.Chunk, md *MeshData) {
	// Máscaras paramétricas dimensionais para: 0=Top, 1=East, 2=West, 3=South, 4=North
	var masks [5][16][16]int32
	colors := make(map[int32]matrix.Color)

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			tile := chunk.Tiles[x][y]
			if tile == nil || tile.Hidden || tile.Shape() == dfproto.ShapeRamp {
				continue
			}

			isWall := tile.IsWall()
			isFloor := tile.IsFloor()
			if !isWall && !isFloor {
				continue
			}

			// Valor de material ajustado +1 (porque 0 significa ignorar quad)
			matVal := materialKey(tile.Material)
			colors[matVal] = getTileColor(tile)

			// 0 = Topo (Pisos/Sólidos SEMPRE desenham a rampa de teto/topo porque fatiamos por camada e a visão topdown exige)
			masks[0][x][y] = matVal

			// Apenas calcular e validar oclusão (Culling lateral inteligente)
			// Apenas paredes reais possuem laterais verticais cheias de 1.0 de altura em Kaiju.
			// Chão no DF é essencialmente uma tampa no nível Z.
			if !isFloor {
				// Face Leste (+X) -> dx=1
				if shouldDrawFace(chunk, x, y, 1, 0, isFloor) {
					masks[1][x][y] = matVal
				}
				// Face Oeste (-X) -> dx=-1
				if shouldDrawFace(chunk, x, y, -1, 0, isFloor) {
					masks[2][x][y] = matVal
				}
				// Face Sul (+Y no mapa do DF) -> dy=1
				if shouldDrawFace(chunk, x, y, 0, 1, isFloor) {
					masks[3][x][y] = matVal
				}
				// Face Norte (-Y no mapa do DF) -> dy=-1
				if shouldDrawFace(chunk, x, y, 0, -1, isFloor) {
					masks[4][x][y] = matVal
				}
			}
		}
	}

	// Iteração Máxima Greedy em 2 Direções (X e Y) sobre cada Face Direcional Limpa de Obstáculos!
	for faceDir := 0; faceDir < 5; faceDir++ {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				matVal := masks[faceDir][x][y]
				if matVal == 0 {
					continue // Pula ocluidos ou processados
				}

				// 1º Passo Euclidiano: Descobrir Largura W máxima de elementos IDÊNTICOS
				w := 1
				canGrowW := true
				if faceDir == 1 || faceDir == 2 {
					canGrowW = false // Laterais Leste/Oeste não expandem lateralmente (X constante)
				}

				if canGrowW {
					for tx := x + 1; tx < 16; tx++ {
						if masks[faceDir][tx][y] == matVal {
							w++
						} else {
							break
						}
					}
				}

				// 2º Passo Euclidiano: Descobrir Altura H máxima repetindo W lisamente
				h := 1
				canGrowH := true
				if faceDir == 3 || faceDir == 4 {
					canGrowH = false // Faces Norte/Sul não expandem em profundidade (Z constante)
				}

				if canGrowH {
					for ty := y + 1; ty < 16; ty++ {
						canExtend := true
						for tx := x; tx < x+w; tx++ {
							if masks[faceDir][tx][ty] != matVal {
								canExtend = false
								break
							}
						}
						if canExtend {
							h++
						} else {
							break
						}
					}
				}

				// Confirmado Retângulo Perfeito: Limpar pixels para os blocos fundidos não travarem subloops
				for ty := y; ty < y+h; ty++ {
					for tx := x; tx < x+w; tx++ {
						masks[faceDir][tx][ty] = 0
					}
				}

				// Enviar retângulo monstruosamente liso para injeção Draw Call
				color := colors[matVal]
				emitGreedyQuad(md, faceDir, matrix.Float(x), matrix.Float(y), matrix.Float(w), matrix.Float(h), color)
			}
		}
	}
}

// emitGreedyQuad é responsável por aplicar os quadros mesclados WxH nas normais devidas do plano da Kaiju Engine.
func emitGreedyQuad(md *MeshData, faceDir int, fx, fy, fw, fh matrix.Float, color matrix.Color) {
	fz := matrix.Float(1.0)

	switch faceDir {
	case 0: // Topo (Up) (+Y do mundo / Positivo Kaiju)
		v0 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -fy), Normal: matrix.Vec3Up(), Color: color}
		v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -fy), Normal: matrix.Vec3Up(), Color: color}
		v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -(fy + fh)), Normal: matrix.Vec3Up(), Color: color}
		v3 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -(fy + fh)), Normal: matrix.Vec3Up(), Color: color}
		// Inverte quad para garantir Winding Order correto e Face voltada para cima!
		md.AddQuad(v0, v1, v2, v3)

	case 1: // Face Direita / East (+X normal)
		v0 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, 0, -fy), Normal: matrix.Vec3Right(), Color: color}
		v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, 0, -(fy + fh)), Normal: matrix.Vec3Right(), Color: color}
		v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -(fy + fh)), Normal: matrix.Vec3Right(), Color: color}
		v3 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -fy), Normal: matrix.Vec3Right(), Color: color}
		md.AddQuad(v0, v1, v2, v3) // Invertido CCW Kaiju (Fix do Right)

	case 2: // Face Esquerda / West (-X normal)
		v0 := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -fy), Normal: matrix.Vec3Left(), Color: color}
		v1 := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -(fy + fh)), Normal: matrix.Vec3Left(), Color: color}
		v2 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -(fy + fh)), Normal: matrix.Vec3Left(), Color: color}
		v3 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -fy), Normal: matrix.Vec3Left(), Color: color}
		md.AddQuad(v3, v2, v1, v0) // Invertido para normal -X real

	case 3: // Face Traseira / South (+Y DF -> -Z Kaiju limite negativo normal Vec3Backward)
		v0 := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -(fy + fh)), Normal: matrix.Vec3Forward(), Color: color}
		v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, 0, -(fy + fh)), Normal: matrix.Vec3Forward(), Color: color}
		v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -(fy + fh)), Normal: matrix.Vec3Forward(), Color: color}
		v3 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -(fy + fh)), Normal: matrix.Vec3Forward(), Color: color}
		// A normal Frontal do Kaiju (-Z Forward) aponta pro lado da tela. Como +YDF cresce para tela(South), o normal é Vec3Forward()
		md.AddQuad(v3, v2, v1, v0)

	case 4: // Face Frontal / North (-Y DF -> Borda Z Kaiju Superior Normal Vec3Backward)
		v0 := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -fy), Normal: matrix.Vec3Backward(), Color: color}
		v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, 0, -fy), Normal: matrix.Vec3Backward(), Color: color}
		v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, -fy), Normal: matrix.Vec3Backward(), Color: color}
		v3 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, -fy), Normal: matrix.Vec3Backward(), Color: color}
		md.AddQuad(v0, v1, v2, v3)
	}
}

// meshSpecials lida com blocos unitários complexos
func meshSpecials(chunk *mapdata.Chunk, md *MeshData) {
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			tile := chunk.Tiles[x][y]
			if tile == nil || tile.Hidden {
				continue
			}

			shape := tile.Shape()
			color := getTileColor(tile)

			fx := matrix.Float(x)
			fy := matrix.Float(y)

			// 1. Rampas
			if shape == dfproto.ShapeRamp {
				rampType := tile.RampType
				geom := GetRampGeometry(rampType)
				if geom != nil {
					// O OBJ modelado tem o centro na base e coordenadas variando de -0.5 a 0.5.
					// Portanto, precisamos transladar para o centro do voxel.
					// O KaijuEngine Voxel é [0..1], então centro = +0.5.
					// eixo Y do df (Kaiju -Z): centro é -(fy + 0.5)
					offset := matrix.NewVec3(fx+0.5, 0, -(fy + 0.5))
					md.AddGeometry(geom.Verts, geom.Indices, offset, color)
				}
			}

			// 2. Escadas (4 Sub-degraus)
			if shape == dfproto.ShapeStairUp || shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
				stepCount := matrix.Float(4.0)
				stepSizeH := matrix.Float(1.0) / stepCount
				stepSizeD := matrix.Float(1.0) / stepCount

				w := matrix.Float(1.0)

				for i := matrix.Float(0.0); i < stepCount; i++ {
					curH := (i + 1) * stepSizeH
					prevH := i * stepSizeH
					// No DF "y" é depth. Os degraus descem ao longo de Y+.
					// Portanto, no Kaiju, a posição Z vai de -fy até -(fy+1)
					curZ := -(fy + (i * stepSizeD))

					// Topo do degrau
					v0t := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ), Normal: matrix.Vec3Up(), Color: color}
					v1t := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ), Normal: matrix.Vec3Up(), Color: color}
					v2t := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ-stepSizeD), Normal: matrix.Vec3Up(), Color: color}
					v3t := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ-stepSizeD), Normal: matrix.Vec3Up(), Color: color}
					md.AddQuad(v0t, v3t, v2t, v1t)

					// Face Frontal do degrau (Encosto vertical)
					v0f := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v1f := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v2f := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v3f := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					md.AddQuad(v3f, v2f, v1f, v0f)
				}

				// Escada descendo tem piso na base
				if shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
					v0b := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -fy), Normal: matrix.Vec3Up(), Color: color}
					v1b := rendering.Vertex{Position: matrix.NewVec3(fx+w, 0, -fy), Normal: matrix.Vec3Up(), Color: color}
					v2b := rendering.Vertex{Position: matrix.NewVec3(fx+w, 0, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
					v3b := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
					md.AddQuad(v0b, v3b, v2b, v1b)
				}
			}
		}
	}
}

// meshLiquids gera planos translúcidos que representam os fluídos
func meshLiquids(chunk *mapdata.Chunk, md *MeshData) {
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			tile := chunk.Tiles[x][y]
			if tile == nil {
				continue
			}

			fx := matrix.Float(x)
			fy := matrix.Float(y)
			fw := matrix.Float(1.0)

			// 1. Água
			if tile.WaterLevel > 0 {
				color := matrix.NewColor(0.0, 0.4, 0.8, 0.6) // Azul translúcido
				h := (matrix.Float(tile.WaterLevel) / 7.0) + 0.05

				v0 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				v3 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				md.AddQuad(v0, v3, v2, v1)
			}

			// 2. Magma
			if tile.MagmaLevel > 0 {
				color := matrix.NewColor(0.8, 0.2, 0.0, 0.9) // Vermelho alaranjado (denso brilhante)
				h := (matrix.Float(tile.MagmaLevel) / 7.0) + 0.05

				v0 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				v3 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				md.AddQuad(v0, v3, v2, v1)
			}
		}
	}
}
