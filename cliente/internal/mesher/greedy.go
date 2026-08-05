package mesher

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
	"math"
)

// materialKey identifica o par completo (tipo + índice), evitando colisões
// entre materiais que compartilham o mesmo MatIndex em categorias diferentes.
func materialKey(pair dfproto.MatPair) int32 {
	const indexBits = 20
	return ((pair.MatType + 1) << indexBits) | (pair.MatIndex + 1)
}

// renderVegetation fica desligado temporariamente para remover árvores,
// copas, galhos e folhas do cliente durante a investigação dos travamentos.
const renderVegetation = false

func isVegetationShape(shape dfproto.TiletypeShape) bool {
	switch shape {
	case dfproto.ShapeTreeShape, dfproto.ShapeSapling, dfproto.ShapeShrub,
		dfproto.ShapeBranch, dfproto.ShapeTrunkBranch, dfproto.ShapeTwig:
		return true
	default:
		return false
	}
}

// getTileColor usa as cores reais recebidas do DFHack. O fallback cinza é
// usado apenas durante a janela em que a lista de materiais ainda não chegou.
func getTileColor(tile *mapdata.Tile) matrix.Color {
	if tile == nil || tile.GetStore() == nil || tile.GetStore().MatStore == nil {
		return matrix.NewColor(0.5, 0.5, 0.5, 1.0)
	}

	matStore := tile.GetStore().MatStore
	color := matStore.GetTileColor(tile)
	// Construções carregam o material colocado pelo jogador em um campo
	// separado do material geológico do tile. Preferi-lo mantém a cor real do
	// DFHack em paredes, rampas e pisos construídos.
	if tile.MaterialCategory() == dfproto.TilematConstruction || tile.ConstructionItem != (dfproto.MatPair{}) {
		if constructionColor, ok := matStore.GetMaterialColor(tile.ConstructionItem); ok {
			color = constructionColor
		}
	}
	return matrix.NewColor(
		matrix.Float(color.R)/255.0,
		matrix.Float(color.G)/255.0,
		matrix.Float(color.B)/255.0,
		matrix.Float(color.A)/255.0,
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
	if !renderVegetation && isVegetationShape(shape) {
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
		if shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification || shape == dfproto.ShapeFloor || shape == dfproto.ShapeTreeShape {
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
			shape := dfproto.ShapeNoShape
			if tile != nil {
				shape = tile.Shape()
			}
			if tile == nil || tile.Hidden || shape == dfproto.ShapeRamp ||
				shape == dfproto.ShapeBoulder || shape == dfproto.ShapePebbles || isVegetationShape(shape) {
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
			if !renderVegetation && isVegetationShape(tile.Shape()) {
				continue
			}

			shape := tile.Shape()
			color := getTileColor(tile)

			fx := matrix.Float(x)
			fy := matrix.Float(y)
			if isVegetationShape(shape) {
				if renderVegetation {
					meshVegetationTile(md, shape, fx, fy, color)
				}
				continue
			}

			// Pedras e pedregulhos não são pisos planos. Se entrarem no greedy,
			// o mesher produz apenas uma tampa no topo do voxel e perde o volume
			// externo. Gerá-los aqui mantém a silhueta 3D e evita z-fighting.
			switch shape {
			case dfproto.ShapeBoulder:
				meshBoulder(md, fx, fy, color)
				continue
			case dfproto.ShapePebbles:
				meshPebbles(md, fx, fy, color)
				continue
			case dfproto.ShapeEndlessPit:
				meshEndlessPit(md, fx, fy, color)
				continue
			}

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

					// Topo do degrau. A altura correta é curH; usar prevH
					// achatava o primeiro degrau no piso.
					v0t := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ), Normal: matrix.Vec3Up(), Color: color}
					v1t := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ), Normal: matrix.Vec3Up(), Color: color}
					v2t := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ-stepSizeD), Normal: matrix.Vec3Up(), Color: color}
					v3t := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ-stepSizeD), Normal: matrix.Vec3Up(), Color: color}
					addDoubleSidedQuad(md, v0t, v3t, v2t, v1t)

					// Face Frontal do degrau (Encosto vertical)
					v0f := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v1f := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v2f := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					v3f := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ-stepSizeD), Normal: matrix.Vec3Forward(), Color: color}
					addDoubleSidedQuad(md, v3f, v2f, v1f, v0f)

					// Laterais externas do degrau. Sem estas faces, a escada
					// ficava aberta quando observada pela esquerda/direita.
					left0 := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ), Normal: matrix.Vec3Left(), Color: color}
					left1 := rendering.Vertex{Position: matrix.NewVec3(fx, prevH, curZ-stepSizeD), Normal: matrix.Vec3Left(), Color: color}
					left2 := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ-stepSizeD), Normal: matrix.Vec3Left(), Color: color}
					left3 := rendering.Vertex{Position: matrix.NewVec3(fx, curH, curZ), Normal: matrix.Vec3Left(), Color: color}
					addDoubleSidedQuad(md, left0, left1, left2, left3)

					right0 := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ-stepSizeD), Normal: matrix.Vec3Right(), Color: color}
					right1 := rendering.Vertex{Position: matrix.NewVec3(fx+w, prevH, curZ), Normal: matrix.Vec3Right(), Color: color}
					right2 := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ), Normal: matrix.Vec3Right(), Color: color}
					right3 := rendering.Vertex{Position: matrix.NewVec3(fx+w, curH, curZ-stepSizeD), Normal: matrix.Vec3Right(), Color: color}
					addDoubleSidedQuad(md, right0, right1, right2, right3)
				}

				// Escada descendo tem piso na base
				if shape == dfproto.ShapeStairDown || shape == dfproto.ShapeStairUpDown {
					v0b := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -fy), Normal: matrix.Vec3Up(), Color: color}
					v1b := rendering.Vertex{Position: matrix.NewVec3(fx+w, 0, -fy), Normal: matrix.Vec3Up(), Color: color}
					v2b := rendering.Vertex{Position: matrix.NewVec3(fx+w, 0, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
					v3b := rendering.Vertex{Position: matrix.NewVec3(fx, 0, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
					addDoubleSidedQuad(md, v0b, v3b, v2b, v1b)
				}
			}
		}
	}
}

// meshBoulder cria uma rocha facetada, pequena o bastante para ficar dentro
// do tile, mas com base, ombros e topo para que ela não pareça um quad plano.
func meshBoulder(md *MeshData, fx, fy matrix.Float, color matrix.Color) {
	meshRock(md, fx+0.5, -(fy + 0.5), 0.47, 0.43, 0.48, 0.0, color)
}

// meshPebbles representa o tile de pedregulhos como três pedras menores. Os
// offsets são fixos para a mesma geometria em qualquer atualização do chunk.
func meshPebbles(md *MeshData, fx, fy matrix.Float, color matrix.Color) {
	cx, cz := fx+0.5, -(fy + 0.5)
	meshRock(md, cx-0.20, cz+0.14, 0.17, 0.14, 0.18, 0.20, color)
	meshRock(md, cx+0.12, cz-0.12, 0.20, 0.16, 0.22, 1.35, color)
	meshRock(md, cx+0.22, cz+0.18, 0.13, 0.12, 0.14, 2.35, color)
}

func meshEndlessPit(md *MeshData, fx, fy matrix.Float, color matrix.Color) {
	const pitDepth matrix.Float = 4.0
	x0, x1 := fx, fx+1
	z0, z1 := -fy, -(fy + 1)
	pitColor := matrix.NewColor(color.R()*0.12, color.G()*0.12, color.B()*0.14, color.A())

	// Paredes internas: o fundo fica abaixo de vários níveis e não tampa a
	// geometria real dos andares inferiores quando eles estiverem carregados.
	addDoubleSidedQuad(md,
		rendering.Vertex{Position: matrix.NewVec3(x0, 0, z0), Normal: matrix.Vec3Left(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, 0, z1), Normal: matrix.Vec3Left(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z1), Normal: matrix.Vec3Left(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z0), Normal: matrix.Vec3Left(), Color: pitColor},
	)
	addDoubleSidedQuad(md,
		rendering.Vertex{Position: matrix.NewVec3(x1, 0, z1), Normal: matrix.Vec3Right(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, 0, z0), Normal: matrix.Vec3Right(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z0), Normal: matrix.Vec3Right(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z1), Normal: matrix.Vec3Right(), Color: pitColor},
	)
	addDoubleSidedQuad(md,
		rendering.Vertex{Position: matrix.NewVec3(x0, 0, z1), Normal: matrix.Vec3Forward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, 0, z1), Normal: matrix.Vec3Forward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z1), Normal: matrix.Vec3Forward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z1), Normal: matrix.Vec3Forward(), Color: pitColor},
	)
	addDoubleSidedQuad(md,
		rendering.Vertex{Position: matrix.NewVec3(x1, 0, z0), Normal: matrix.Vec3Backward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, 0, z0), Normal: matrix.Vec3Backward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z0), Normal: matrix.Vec3Backward(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z0), Normal: matrix.Vec3Backward(), Color: pitColor},
	)
	addDoubleSidedQuad(md,
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z0), Normal: matrix.Vec3Up(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x0, -pitDepth, z1), Normal: matrix.Vec3Up(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z1), Normal: matrix.Vec3Up(), Color: pitColor},
		rendering.Vertex{Position: matrix.NewVec3(x1, -pitDepth, z0), Normal: matrix.Vec3Up(), Color: pitColor},
	)
}

// meshRock emite um elipsoide low-poly com faces duplicadas. A duplicação é
// intencional: o material básico usa back-face culling e a rocha precisa
// continuar visível quando a câmera passa para qualquer lado do voxel.
func meshRock(md *MeshData, cx, cz, radiusX, radiusZ, height, phase matrix.Float, color matrix.Color) {
	const segments = 8
	baseY := matrix.Float(0.015)
	base := make([]rendering.Vertex, segments)
	lower := make([]rendering.Vertex, segments)
	upper := make([]rendering.Vertex, segments)

	for i := 0; i < segments; i++ {
		angle := phase + matrix.Float(2*math.Pi*float64(i)/segments)
		cosA := matrix.Float(math.Cos(float64(angle)))
		sinA := matrix.Float(math.Sin(float64(angle)))
		base[i] = rockVertex(cx+cosA*radiusX*0.72, baseY, cz+sinA*radiusZ*0.72, cx, cz, radiusX, radiusZ, height, color)
		lower[i] = rockVertex(cx+cosA*radiusX, height*0.34, cz+sinA*radiusZ, cx, cz, radiusX, radiusZ, height, color)
		upper[i] = rockVertex(cx+cosA*radiusX*0.63, height*0.77, cz+sinA*radiusZ*0.63, cx, cz, radiusX, radiusZ, height, color)
	}

	top := rockVertex(cx, height, cz, cx, cz, radiusX, radiusZ, height, color)
	bottom := rockVertex(cx, 0, cz, cx, cz, radiusX, radiusZ, height, color)
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments

		addDoubleSidedTriangle(md, top, upper[next], upper[i])
		addDoubleSidedTriangle(md, upper[i], lower[next], lower[i])
		addDoubleSidedTriangle(md, upper[i], upper[next], lower[next])
		addDoubleSidedTriangle(md, lower[i], base[next], base[i])
		addDoubleSidedTriangle(md, lower[i], lower[next], base[next])
		addDoubleSidedTriangle(md, bottom, base[i], base[next])
	}
}

func rockVertex(x, y, z, cx, cz, radiusX, radiusZ, height matrix.Float, color matrix.Color) rendering.Vertex {
	// Normal elipsoidal aproximada: dá um sombreado contínuo sem precisar de
	// uma malha densa, mantendo o aspecto low-poly nas arestas.
	nx := (x - cx) / radiusX
	ny := (y - height*0.48) / (height * 0.55)
	nz := (z - cz) / radiusZ
	length := matrix.Float(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
	if length <= 0 {
		length = 1
	}
	return rendering.Vertex{
		Position: matrix.NewVec3(x, y, z),
		Normal:   matrix.NewVec3(nx/length, ny/length, nz/length),
		Color:    color,
	}
}

func addDoubleSidedTriangle(md *MeshData, v0, v1, v2 rendering.Vertex) {
	md.AddTriangle(v0, v1, v2)
	v0.Normal = negateNormal(v0.Normal)
	v1.Normal = negateNormal(v1.Normal)
	v2.Normal = negateNormal(v2.Normal)
	md.AddTriangle(v0, v2, v1)
}

func addDoubleSidedQuad(md *MeshData, v0, v1, v2, v3 rendering.Vertex) {
	md.AddQuad(v0, v1, v2, v3)
	v0.Normal = negateNormal(v0.Normal)
	v1.Normal = negateNormal(v1.Normal)
	v2.Normal = negateNormal(v2.Normal)
	v3.Normal = negateNormal(v3.Normal)
	md.AddQuad(v3, v2, v1, v0)
}

func meshVegetationTile(md *MeshData, shape dfproto.TiletypeShape, fx, fy matrix.Float, color matrix.Color) {
	center := matrix.NewVec3(fx+0.5, 0, -(fy + 0.5))
	leafColor := matrix.NewColor(color.R()*0.75, color.G()*1.05, color.B()*0.65, color.A())
	if leafColor.G() > 1 {
		leafColor = matrix.NewColor(leafColor.R(), 1, leafColor.B(), leafColor.A())
	}

	switch shape {
	case dfproto.ShapeTreeShape:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.7, 0)), matrix.NewVec3(0.9, 0.65, 0.9), leafColor)
	case dfproto.ShapeTrunkBranch:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.35, 0)), matrix.NewVec3(0.24, 0.7, 0.24), color)
	case dfproto.ShapeBranch:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.45, 0)), matrix.NewVec3(0.75, 0.16, 0.16), color)
	case dfproto.ShapeTwig:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.55, 0)), matrix.NewVec3(0.3, 0.12, 0.3), leafColor)
	case dfproto.ShapeSapling:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.22, 0)), matrix.NewVec3(0.12, 0.44, 0.12), color)
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.5, 0)), matrix.NewVec3(0.42, 0.35, 0.42), leafColor)
	case dfproto.ShapeShrub:
		addVegetationBox(md, center.Add(matrix.NewVec3(0, 0.28, 0)), matrix.NewVec3(0.75, 0.5, 0.75), leafColor)
	}
}

func addVegetationBox(md *MeshData, center, size matrix.Vec3, color matrix.Color) {
	hx, hy, hz := size.X()*0.5, size.Y()*0.5, size.Z()*0.5
	x, y, z := center.X(), center.Y(), center.Z()
	bottom := y - hy
	top := y + hy
	front := z + hz
	back := z - hz
	left := x - hx
	right := x + hx
	normalUp := matrix.Vec3Up()
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(left, top, front), Normal: normalUp, Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, front), Normal: normalUp, Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, back), Normal: normalUp, Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, top, back), Normal: normalUp, Color: color},
	)
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, back), Normal: matrix.Vec3Down(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, back), Normal: matrix.Vec3Down(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, front), Normal: matrix.Vec3Down(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, front), Normal: matrix.Vec3Down(), Color: color},
	)
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, front), Normal: matrix.Vec3Forward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, front), Normal: matrix.Vec3Forward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, front), Normal: matrix.Vec3Forward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, top, front), Normal: matrix.Vec3Forward(), Color: color},
	)
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, back), Normal: matrix.Vec3Backward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, back), Normal: matrix.Vec3Backward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, top, back), Normal: matrix.Vec3Backward(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, back), Normal: matrix.Vec3Backward(), Color: color},
	)
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, front), Normal: matrix.Vec3Right(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, bottom, back), Normal: matrix.Vec3Right(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, back), Normal: matrix.Vec3Right(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(right, top, front), Normal: matrix.Vec3Right(), Color: color},
	)
	md.AddQuad(
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, back), Normal: matrix.Vec3Left(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, bottom, front), Normal: matrix.Vec3Left(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, top, front), Normal: matrix.Vec3Left(), Color: color},
		rendering.Vertex{Position: matrix.NewVec3(left, top, back), Normal: matrix.Vec3Left(), Color: color},
	)
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
			surfaceHeight := matrix.Float(0.06)

			// 1. Água
			if tile.WaterLevel > 0 {
				color := matrix.NewColor(0.0, 0.4, 0.8, 0.6) // Azul translúcido
				h := (matrix.Float(tile.WaterLevel) / 7.0) + 0.05
				surfaceHeight = h

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
				if h > surfaceHeight {
					surfaceHeight = h
				}

				v0 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -fy), Normal: matrix.Vec3Up(), Color: color}
				v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				v3 := rendering.Vertex{Position: matrix.NewVec3(fx, h, -(fy + 1)), Normal: matrix.Vec3Up(), Color: color}
				md.AddQuad(v0, v3, v2, v1)
			}

			if (tile.WaterLevel > 0 || tile.MagmaLevel > 0) &&
				(tile.FlowVector.X != 0 || tile.FlowVector.Y != 0) {
				flowColor := matrix.NewColor(0.55, 0.95, 1.0, 0.95)
				if tile.MagmaLevel > 0 {
					flowColor = matrix.NewColor(1.0, 0.85, 0.25, 0.95)
				}
				addFlowArrow(md, fx, fy, surfaceHeight+0.02, tile.FlowVector, flowColor)
			}
		}
	}
}

func addFlowArrow(md *MeshData, fx, fy, height matrix.Float, flow util.DFCoord, color matrix.Color) {
	dx := matrix.Float(flow.X)
	dz := -matrix.Float(flow.Y)
	if dx == 0 && dz == 0 {
		return
	}
	// O vetor recebido é cardinal/diagonal; reduzir para uma seta uniforme.
	if dx != 0 {
		dx /= matrix.Float(util.Abs(flow.X))
	}
	if dz != 0 {
		dz /= matrix.Float(util.Abs(flow.Y))
	}
	px, pz := -dz, dx
	cx, cz := fx+0.5, -(fy + 0.5)
	left := matrix.NewVec3(cx-dx*0.2-px*0.16, height, cz-dz*0.2-pz*0.16)
	right := matrix.NewVec3(cx-dx*0.2+px*0.16, height, cz-dz*0.2+pz*0.16)
	tip := matrix.NewVec3(cx+dx*0.34, height, cz+dz*0.34)
	normal := matrix.Vec3Up()
	md.AddTriangle(
		rendering.Vertex{Position: left, Normal: normal, Color: color},
		rendering.Vertex{Position: right, Normal: normal, Color: color},
		rendering.Vertex{Position: tip, Normal: normal, Color: color},
	)
}
