package mesher

import (
	"fmt"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

type CachedGeometry struct {
	Verts   []rendering.Vertex
	Indices []uint32
}

var rampCache = make(map[int32]*CachedGeometry)

// LoadRampGeometries deve ser invocado no boot para parsear os arquivos OBJ
// de forma independente do KaijuEngine (evitando crash por dependência de VFS).
func LoadRampGeometries(basePath string) {
	log.Println("⛰️ [Mesher] Pré-carregando malhas OBJ de rampas com parser nativo...")

	sucessos := 0
	assetRoots := rampAssetRoots(basePath)

	for i := 1; i <= 26; i++ {
		loaded := false
		for _, root := range assetRoots {
			paths := []string{
				fmt.Sprintf("%s/assets/models/ramps/RAMP_%d.obj", root, i),
				fmt.Sprintf("%s/assets/models/ramps/RAMP_%d_blunt.obj", root, i),
				fmt.Sprintf("%s/assets/models/ramps/RAMP_%d_sharp.obj", root, i),
			}
			for _, p := range paths {
				geom := loadSimpleObj(p)
				if geom != nil {
					rampCache[int32(i)] = geom
					sucessos++
					loaded = true
					break
				}
			}
			if loaded {
				break
			}
		}
	}

	// Rampa "UP"
	for _, root := range assetRoots {
		pathUp := fmt.Sprintf("%s/assets/models/ramps/RAMP_UP.obj", root)
		geomUp := loadSimpleObj(pathUp)
		if geomUp != nil {
			rampCache[99] = geomUp
			sucessos++
			break
		}
	}

	log.Printf("⛰️ [Mesher] %d rampas carregadas na RAM com sucesso.", sucessos)
}

func GetRampGeometry(rampType int32) *CachedGeometry {
	if geom, ok := rampCache[rampType]; ok {
		return geom
	}
	if geom, ok := rampCache[1]; ok {
		return geom
	}
	return nil
}

func rampAssetRoots(basePath string) []string {
	if basePath == "" {
		basePath = "."
	}
	roots := []string{basePath}
	if basePath == "." {
		roots = append(roots, "cliente")
	}
	return roots
}

// loadSimpleObj extrai triangulos de um OBJ basico no formato v, vn e f v//vn
func loadSimpleObj(path string) *CachedGeometry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")

	var positions []matrix.Vec3
	var normals []matrix.Vec3
	var minY, maxY matrix.Float
	haveY := false

	geom := &CachedGeometry{
		Verts:   make([]rendering.Vertex, 0),
		Indices: make([]uint32, 0),
	}

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) == 0 || l[0] == '#' {
			continue
		}

		parts := strings.Fields(l)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "v":
			if len(parts) >= 4 {
				x, _ := strconv.ParseFloat(parts[1], 32)
				y, _ := strconv.ParseFloat(parts[2], 32)
				z, _ := strconv.ParseFloat(parts[3], 32)
				pos := matrix.NewVec3(matrix.Float(x)*0.5, matrix.Float(y)*0.5, matrix.Float(z)*0.5)
				positions = append(positions, pos)
				if !haveY || pos.Y() < minY {
					minY = pos.Y()
				}
				if !haveY || pos.Y() > maxY {
					maxY = pos.Y()
				}
				haveY = true
			}
		case "vn":
			if len(parts) >= 4 {
				x, _ := strconv.ParseFloat(parts[1], 32)
				y, _ := strconv.ParseFloat(parts[2], 32)
				z, _ := strconv.ParseFloat(parts[3], 32)
				normals = append(normals, matrix.NewVec3(matrix.Float(x), matrix.Float(y), matrix.Float(z)))
			}
		case "f":
			// Processar faces (podem ser triângulos ou quads)
			// Face formato: v/vt/vn ou v//vn ou v
			var faceIndices []uint32

			for i := 1; i < len(parts); i++ {
				subParts := strings.Split(parts[i], "/")
				vIdx, _ := strconv.Atoi(subParts[0])
				vIdx-- // OBJ é 1-indexed

				nIdx := vIdx
				if len(subParts) >= 3 && subParts[2] != "" {
					parsedN, _ := strconv.Atoi(subParts[2])
					nIdx = parsedN - 1
				}

				// Pegar os vetores
				pos := positions[vIdx]
				// Os OBJ originais misturam alturas de 2.0 e 3.5 unidades,
				// enquanto o terreno do cliente tem exatamente 1 unidade por Z.
				// Normalizar somente o eixo vertical evita que a rampa atravesse
				// o piso superior e apareça como uma parede/espinho.
				if haveY && maxY > minY {
					pos = matrix.NewVec3(pos.X(), (pos.Y()-minY)/(maxY-minY), pos.Z())
				}
				var norm matrix.Vec3
				if nIdx < len(normals) && nIdx >= 0 {
					norm = normals[nIdx]
				} else {
					norm = matrix.Vec3Up()
				}

				newVert := rendering.Vertex{Position: pos, Normal: norm, Color: matrix.ColorWhite()}
				geom.Verts = append(geom.Verts, newVert)
				faceIndices = append(faceIndices, uint32(len(geom.Verts)-1))
			}

			// Os OBJ já vêm com o winding voltado para fora. Preservar a ordem
			// original é necessário para o back-face culling não esconder as faces
			// externas da rampa.
			if len(faceIndices) == 3 {
				geom.Indices = append(geom.Indices, faceIndices[0], faceIndices[1], faceIndices[2])
			} else if len(faceIndices) == 4 {
				geom.Indices = append(geom.Indices, faceIndices[0], faceIndices[1], faceIndices[2])
				geom.Indices = append(geom.Indices, faceIndices[0], faceIndices[2], faceIndices[3])
			} else if len(faceIndices) > 4 {
				// Polygon genérico com triangulação em leque.
				v0 := faceIndices[0]
				for k := 1; k < len(faceIndices)-1; k++ {
					geom.Indices = append(geom.Indices, v0, faceIndices[k], faceIndices[k+1])
				}
			}
		}
	}

	if len(geom.Verts) == 0 {
		return nil
	}

	// Alguns exportadores OBJ deixam a casca lateral dependente da ordem das
	// faces. Reconstituir as quatro faces externas a partir dos quatro cantos
	// da base torna a rampa um volume fechado mesmo quando uma dessas faces
	// chega invertida, incompleta ou é descartada pelo rasterizador.
	rebuildRampExternalShell(geom)

	// As rampas precisam ser observáveis pelo lado de fora e pelo lado de
	// dentro. O material básico do cliente usa back-face culling; como os OBJ
	// vieram de exportadores diferentes, algumas faces ficam voltadas para o
	// sentido oposto ao esperado pela câmera. Duplicar os triângulos com
	// winding e normais invertidos evita que a parte externa desapareça quando
	// a câmera passa para o outro lado da rampa.
	makeGeometryDoubleSided(geom)
	return geom
}

func rebuildRampExternalShell(geom *CachedGeometry) {
	if geom == nil || len(geom.Verts) < 4 || len(geom.Indices) < 3 {
		return
	}

	minX, maxX := geom.Verts[0].Position.X(), geom.Verts[0].Position.X()
	minY, maxY := geom.Verts[0].Position.Y(), geom.Verts[0].Position.Y()
	minZ, maxZ := geom.Verts[0].Position.Z(), geom.Verts[0].Position.Z()
	for _, vertex := range geom.Verts[1:] {
		position := vertex.Position
		minX = minFloat(minX, position.X())
		maxX = maxFloat(maxX, position.X())
		minY = minFloat(minY, position.Y())
		maxY = maxFloat(maxY, position.Y())
		minZ = minFloat(minZ, position.Z())
		maxZ = maxFloat(maxZ, position.Z())
	}

	const tolerance matrix.Float = 0.0001
	if maxX-minX < tolerance || maxZ-minZ < tolerance || maxY-minY < tolerance {
		return
	}

	// Só trata como rampa um volume que realmente possui os quatro cantos da
	// base. Isso mantém o parser genérico para OBJ de teste ou outros modelos.
	for _, corner := range [][2]matrix.Float{
		{minX, minZ}, {minX, maxZ}, {maxX, minZ}, {maxX, maxZ},
	} {
		if !hasRampBaseCorner(geom.Verts, corner[0], corner[1], minY, tolerance) {
			return
		}
	}

	cornerHeight := func(x, z matrix.Float) matrix.Float {
		height := minY
		for _, vertex := range geom.Verts {
			position := vertex.Position
			if almostEqual(position.X(), x, tolerance) && almostEqual(position.Z(), z, tolerance) {
				height = maxFloat(height, position.Y())
			}
		}
		return height
	}

	// Remove somente as faces verticais exportadas. As faces inclinadas do
	// topo e a base continuam intactas; as laterais serão inseridas abaixo
	// com winding e normais conhecidos.
	originalVerts := geom.Verts
	originalIndices := geom.Indices
	geom.Verts = make([]rendering.Vertex, 0, len(originalVerts)+16)
	geom.Indices = make([]uint32, 0, len(originalIndices)+24)
	for i := 0; i+2 < len(originalIndices); i += 3 {
		i0, i1, i2 := originalIndices[i], originalIndices[i+1], originalIndices[i+2]
		if int(i0) >= len(originalVerts) || int(i1) >= len(originalVerts) || int(i2) >= len(originalVerts) {
			continue
		}
		v0, v1, v2 := originalVerts[i0], originalVerts[i1], originalVerts[i2]
		if rampExternalTriangle(v0.Position, v1.Position, v2.Position, minX, maxX, minZ, maxZ, tolerance) {
			continue
		}
		start := uint32(len(geom.Verts))
		geom.Verts = append(geom.Verts, v0, v1, v2)
		geom.Indices = append(geom.Indices, start, start+1, start+2)
	}

	minZMinX := cornerHeight(minX, minZ)
	maxZMinX := cornerHeight(minX, maxZ)
	minZMaxX := cornerHeight(maxX, minZ)
	maxZMaxX := cornerHeight(maxX, maxZ)

	// Oeste/Leste (-X/+X).
	addRampExternalQuad(geom,
		matrix.NewVec3(minX, minY, minZ), matrix.NewVec3(minX, minY, maxZ),
		matrix.NewVec3(minX, maxZMinX, maxZ), matrix.NewVec3(minX, minZMinX, minZ),
		matrix.Vec3Left())
	addRampExternalQuad(geom,
		matrix.NewVec3(maxX, minY, maxZ), matrix.NewVec3(maxX, minY, minZ),
		matrix.NewVec3(maxX, minZMaxX, minZ), matrix.NewVec3(maxX, maxZMaxX, maxZ),
		matrix.Vec3Right())

	// Norte/Sul (-Z/+Z).
	addRampExternalQuad(geom,
		matrix.NewVec3(maxX, minY, minZ), matrix.NewVec3(minX, minY, minZ),
		matrix.NewVec3(minX, minZMinX, minZ), matrix.NewVec3(maxX, minZMaxX, minZ),
		matrix.Vec3Forward())
	addRampExternalQuad(geom,
		matrix.NewVec3(minX, minY, maxZ), matrix.NewVec3(maxX, minY, maxZ),
		matrix.NewVec3(maxX, maxZMaxX, maxZ), matrix.NewVec3(minX, maxZMinX, maxZ),
		matrix.Vec3Backward())
}

func hasRampBaseCorner(verts []rendering.Vertex, x, z, baseY, tolerance matrix.Float) bool {
	for _, vertex := range verts {
		position := vertex.Position
		if almostEqual(position.X(), x, tolerance) &&
			almostEqual(position.Z(), z, tolerance) &&
			position.Y() <= baseY+tolerance {
			return true
		}
	}
	return false
}

func rampExternalTriangle(a, b, c matrix.Vec3, minX, maxX, minZ, maxZ, tolerance matrix.Float) bool {
	// O componente Y do produto vetorial é zero em uma face vertical. Faces
	// de topo e a tampa inferior permanecem, porque possuem normal vertical.
	ax, az := b.X()-a.X(), b.Z()-a.Z()
	bx, bz := c.X()-a.X(), c.Z()-a.Z()
	crossY := az*bx - ax*bz
	if matrix.Float(math.Abs(float64(crossY))) > tolerance ||
		matrix.Float(math.Abs(float64(b.Y()-a.Y()))) <= tolerance &&
			matrix.Float(math.Abs(float64(c.Y()-a.Y()))) <= tolerance {
		return false
	}

	sameX := (almostEqual(a.X(), b.X(), tolerance) && almostEqual(b.X(), c.X(), tolerance)) &&
		(almostEqual(a.X(), minX, tolerance) || almostEqual(a.X(), maxX, tolerance))
	sameZ := (almostEqual(a.Z(), b.Z(), tolerance) && almostEqual(b.Z(), c.Z(), tolerance)) &&
		(almostEqual(a.Z(), minZ, tolerance) || almostEqual(a.Z(), maxZ, tolerance))
	return sameX || sameZ
}

func addRampExternalQuad(geom *CachedGeometry, p0, p1, p2, p3 matrix.Vec3, normal matrix.Vec3) {
	start := uint32(len(geom.Verts))
	geom.Verts = append(geom.Verts,
		rendering.Vertex{Position: p0, Normal: normal, Color: matrix.ColorWhite()},
		rendering.Vertex{Position: p1, Normal: normal, Color: matrix.ColorWhite()},
		rendering.Vertex{Position: p2, Normal: normal, Color: matrix.ColorWhite()},
		rendering.Vertex{Position: p3, Normal: normal, Color: matrix.ColorWhite()},
	)
	geom.Indices = append(geom.Indices, start, start+1, start+2, start, start+2, start+3)
}

func almostEqual(a, b, tolerance matrix.Float) bool {
	return matrix.Float(math.Abs(float64(a-b))) <= tolerance
}

func minFloat(a, b matrix.Float) matrix.Float {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b matrix.Float) matrix.Float {
	if a > b {
		return a
	}
	return b
}

func makeGeometryDoubleSided(geom *CachedGeometry) {
	if geom == nil || len(geom.Indices) < 3 {
		return
	}

	original := append([]uint32(nil), geom.Indices...)
	for i := 0; i+2 < len(original); i += 3 {
		i0, i1, i2 := original[i], original[i+1], original[i+2]
		if int(i0) >= len(geom.Verts) || int(i1) >= len(geom.Verts) || int(i2) >= len(geom.Verts) {
			continue
		}

		v0 := geom.Verts[i0]
		v1 := geom.Verts[i1]
		v2 := geom.Verts[i2]
		v0.Normal = negateNormal(v0.Normal)
		v1.Normal = negateNormal(v1.Normal)
		v2.Normal = negateNormal(v2.Normal)

		start := uint32(len(geom.Verts))
		// A ordem invertida é a face oposta do mesmo triângulo.
		geom.Verts = append(geom.Verts, v0, v2, v1)
		geom.Indices = append(geom.Indices, start, start+1, start+2)
	}
}

func negateNormal(normal matrix.Vec3) matrix.Vec3 {
	return matrix.NewVec3(-normal.X(), -normal.Y(), -normal.Z())
}
