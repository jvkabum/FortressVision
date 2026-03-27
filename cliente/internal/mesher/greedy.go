package mesher

import (
	"FortressVision/shared/mapdata"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
	"log"
)

// GenerateChunkMesh transforma o voxel-grid do chunk em uma malha 3D otimizada.
func GenerateChunkMesh(chunk *mapdata.Chunk) *ChunkMesh {
	// DF(X, Y, Z) -> Kaiju(X, Z, Y) onde Z é altitude no DF e Y no Kaiju
	origin := matrix.NewVec3(matrix.Float(chunk.Origin.X), matrix.Float(chunk.Origin.Z), matrix.Float(chunk.Origin.Y))
	
	cm := &ChunkMesh{
		Origin:    origin,
		SubMeshes: make(map[int32]*MeshData),
	}

	// DIAGNÓSTICO DE EMERGÊNCIA
	shapeCounts := make(map[int]int)
	matCounts := make(map[int32]int)
	solidsFound := 0
	if len(chunk.Tiles) > 0 {
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				t := chunk.Tiles[x][y]
				if t != nil {
					shape := int(t.Shape())
					shapeCounts[shape]++
					matCounts[t.Material.MatIndex]++
					if shape != 0 && shape != -1 {
						solidsFound++
					}
				}
			}
		}
		t0 := chunk.Tiles[0][0]
		name := "Unknown"
		if tt, ok := t0.GetStore().Tiletypes[t0.TileType]; ok {
			name = tt.Name
		}
		log.Printf("🧪 [Mesher-Stat] Chunk %v: Solids=%d | Shapes=%v | Mats=%v | Tile(0,0): %s (ID=%d, Mat=%d)", 
			chunk.Origin, solidsFound, shapeCounts, matCounts, name, t0.TileType, t0.Material.MatIndex)
	}

	// 1. Mesh de Topo (Floors) - Otimizado com Greedy
	meshTop(chunk, cm)

	// 2. Mesh de Paredes (Walls)
	meshWalls(chunk, cm)

	return cm
}

func meshTop(chunk *mapdata.Chunk, cm *ChunkMesh) {
	processed := [16][16]bool{}

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if processed[x][y] {
				continue
			}

			tile := chunk.Tiles[x][y]
			if tile == nil || !tile.IsFloor() {
				processed[x][y] = true
				continue
			}

			material := tile.Material.MatIndex
			
			w := 1
			for x2 := x + 1; x2 < 16; x2++ {
				t2 := chunk.Tiles[x2][y]
				if !processed[x2][y] && t2 != nil && t2.IsFloor() && t2.Material.MatIndex == material {
					w++
				} else {
					break
				}
			}

			h := 1
			for y2 := y + 1; y2 < 16; y2++ {
				canExpandH := true
				for x2 := x; x2 < x+w; x2++ {
					t2 := chunk.Tiles[x2][y2]
					if processed[x2][y2] || t2 == nil || !t2.IsFloor() || t2.Material.MatIndex != material {
						canExpandH = false
						break
					}
				}
				if canExpandH {
					h++
				} else {
					break
				}
			}

			md, ok := cm.SubMeshes[material]
			if !ok {
				md = NewMeshData()
				cm.SubMeshes[material] = md
			}

			// Adicionar Quad
			fz := matrix.Float(0.1)
			fx := matrix.Float(x)
			fy := matrix.Float(y)
			fw := matrix.Float(w)
			fh := matrix.Float(h)
			
			v0 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, fy), Normal: matrix.Vec3Up()}
			v1 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, fy), Normal: matrix.Vec3Up()}
			v2 := rendering.Vertex{Position: matrix.NewVec3(fx+fw, fz, fy+fh), Normal: matrix.Vec3Up()}
			v3 := rendering.Vertex{Position: matrix.NewVec3(fx, fz, fy+fh), Normal: matrix.Vec3Up()}
			
			md.AddQuad(v0, v1, v2, v3)

			for dy := 0; dy < h; dy++ {
				for dx := 0; dx < w; dx++ {
					processed[x+dx][y+dy] = true
				}
			}
		}
	}
}

func meshWalls(chunk *mapdata.Chunk, cm *ChunkMesh) {
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			tile := chunk.Tiles[x][y]
			if tile == nil || !tile.IsWall() {
				continue
			}

			material := tile.Material.MatIndex
			md, ok := cm.SubMeshes[material]
			if !ok {
				md = NewMeshData()
				cm.SubMeshes[material] = md
			}

			// Adicionar Cubo de Parede (Simplificado: 4 faces laterais + topo)
			// DF(X, Y, Z) -> Kaiju(X, Z, Y)
			fx, fy, fz := matrix.Float(x), matrix.Float(y), matrix.Float(1.0)
			
			// Topo da Parede
			v0t := rendering.Vertex{Position: matrix.NewVec3(fx, fz, fy), Normal: matrix.Vec3Up()}
			v1t := rendering.Vertex{Position: matrix.NewVec3(fx+1, fz, fy), Normal: matrix.Vec3Up()}
			v2t := rendering.Vertex{Position: matrix.NewVec3(fx+1, fz, fy+1), Normal: matrix.Vec3Up()}
			v3t := rendering.Vertex{Position: matrix.NewVec3(fx, fz, fy+1), Normal: matrix.Vec3Up()}
			md.AddQuad(v0t, v1t, v2t, v3t)

			// Face Sul (-Y no DF -> -Z no Kaiju?)
			// Kaiju: X=fx..fx+1, Y=0..1, Z=fy
			v0s := rendering.Vertex{Position: matrix.NewVec3(fx, 0, fy), Normal: matrix.NewVec3(0, 0, -1)}
			v1s := rendering.Vertex{Position: matrix.NewVec3(fx+1, 0, fy), Normal: matrix.NewVec3(0, 0, -1)}
			v2s := rendering.Vertex{Position: matrix.NewVec3(fx+1, 1, fy), Normal: matrix.NewVec3(0, 0, -1)}
			v3s := rendering.Vertex{Position: matrix.NewVec3(fx, 1, fy), Normal: matrix.NewVec3(0, 0, -1)}
			md.AddQuad(v0s, v1s, v2s, v3s)

			// Face Norte
			v0n := rendering.Vertex{Position: matrix.NewVec3(fx, 0, fy+1), Normal: matrix.NewVec3(0, 0, 1)}
			v1n := rendering.Vertex{Position: matrix.NewVec3(fx+1, 0, fy+1), Normal: matrix.NewVec3(0, 0, 1)}
			v2n := rendering.Vertex{Position: matrix.NewVec3(fx+1, 1, fy+1), Normal: matrix.NewVec3(0, 0, 1)}
			v3n := rendering.Vertex{Position: matrix.NewVec3(fx, 1, fy+1), Normal: matrix.NewVec3(0, 0, 1)}
			md.AddQuad(v0n, v1n, v2n, v3n)

			// Face Oeste
			v0w := rendering.Vertex{Position: matrix.NewVec3(fx, 0, fy), Normal: matrix.NewVec3(-1, 0, 0)}
			v1w := rendering.Vertex{Position: matrix.NewVec3(fx, 0, fy+1), Normal: matrix.NewVec3(-1, 0, 0)}
			v2w := rendering.Vertex{Position: matrix.NewVec3(fx, 1, fy+1), Normal: matrix.NewVec3(-1, 0, 0)}
			v3w := rendering.Vertex{Position: matrix.NewVec3(fx, 1, fy), Normal: matrix.NewVec3(-1, 0, 0)}
			md.AddQuad(v0w, v1w, v2w, v3w)

			// Face Leste
			v0e := rendering.Vertex{Position: matrix.NewVec3(fx+1, 0, fy), Normal: matrix.NewVec3(1, 0, 0)}
			v1e := rendering.Vertex{Position: matrix.NewVec3(fx+1, 0, fy+1), Normal: matrix.NewVec3(1, 0, 0)}
			v2e := rendering.Vertex{Position: matrix.NewVec3(fx+1, 1, fy+1), Normal: matrix.NewVec3(1, 0, 0)}
			v3e := rendering.Vertex{Position: matrix.NewVec3(fx+1, 1, fy), Normal: matrix.NewVec3(1, 0, 0)}
			md.AddQuad(v0e, v1e, v2e, v3e)
		}
	}
}
