package mesher

import (
	"fmt"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
	"log"
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

	for i := 1; i <= 26; i++ {
		paths := []string{
			fmt.Sprintf("%s/assets/models/ramps/RAMP_%d.obj", basePath, i),
			fmt.Sprintf("%s/assets/models/ramps/RAMP_%d_blunt.obj", basePath, i),
			fmt.Sprintf("%s/assets/models/ramps/RAMP_%d_sharp.obj", basePath, i),
		}

		for _, p := range paths {
			geom := loadSimpleObj(p)
			if geom != nil {
				rampCache[int32(i)] = geom
				sucessos++
				break
			}
		}
	}

	// Rampa "UP"
	pathUp := fmt.Sprintf("%s/assets/models/ramps/RAMP_UP.obj", basePath)
	geomUp := loadSimpleObj(pathUp)
	if geomUp != nil {
		rampCache[99] = geomUp
		sucessos++
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
	return geom
}
