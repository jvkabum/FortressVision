package util

import (
	"math"
)

// RaycastVoxels percorre os voxels ao longo de um raio até encontrar um hit.
// Usa o algoritmo DDA (Digital Differential Analyzer) rápido para voxels.
func RaycastVoxels(origin, direction Vector3, maxDist float32, hitFunc func(DFCoord) bool) (bool, DFCoord, Vector3) {
	// Ray direction normalization (if not already)
	mag := float32(math.Sqrt(float64(direction.X*direction.X + direction.Y*direction.Y + direction.Z*direction.Z)))
	if mag < 0.0001 {
		return false, DFCoord{}, Vector3{}
	}
	dx, dy, dz := direction.X/mag, direction.Y/mag, direction.Z/mag

	// Mapeamento de Mundi-3D para DF-Coords
	// Mundo(X, Y-up, Z) -> DF(X, -Z, Y-up) conforme nossa calibração
	// Mas o RaycastVoxels costuma trabalhar no espaço da grade. 
	// Vamos trabalhar no espaço DF diretamente.
	
	// Posição inicial no espaço DF
	current := WorldToDFCoord(origin)

	stepX, stepY, stepZ := int32(1), int32(1), int32(1)
	if dx < 0 { stepX = -1 }
	if dy < 0 { stepZ = -1 } // Mundo-Y -> DF-Z
	if dz < 0 { stepY = 1 }  // Mundo-Z -> DF-Y invertido. Se Z < 0 (forward), DF-Y aumenta.

	// Delta distance: quanto o raio percorre para cruzar um voxel em cada eixo
	deltaX := float32(math.Abs(1.0 / float64(dx)))
	deltaY := float32(math.Abs(1.0 / float64(dz))) // Mundo-Z -> DF-Y
	deltaZ := float32(math.Abs(1.0 / float64(dy))) // Mundo-Y -> DF-Z

	// Max distance to next boundary
	var maxX, maxY, maxZ float32
	// DF Coord X
	if dx > 0 {
		maxX = (float32(current.X+1) - origin.X) * deltaX
	} else {
		maxX = (origin.X - float32(current.X)) * deltaX
	}
	// DF Coord Y (Mundo-Z)
	worldZ := -origin.Z
	if dz < 0 { // Forward (Z negativo) em Kaiju -> Norte (Y diminui no DF) ? 
		// Vamos simplificar: usar as coordenadas do mundo transformadas
		maxY = (float32(current.Y+1) - worldZ) * deltaY
	} else {
		maxY = (worldZ - float32(current.Y)) * deltaY
	}
	// DF Coord Z (Mundo-Y)
	if dy > 0 {
		maxZ = (float32(current.Z+1) - origin.Y) * deltaZ
	} else {
		maxZ = (origin.Y - float32(current.Z)) * deltaZ
	}

	dist := float32(0)
	var lastNormal Vector3

	for dist < maxDist {
		if hitFunc(current) {
			return true, current, lastNormal
		}

		if maxX < maxY {
			if maxX < maxZ {
				dist = maxX
				maxX += deltaX
				current.X += stepX
				lastNormal = Vector3{X: float32(-stepX), Y: 0, Z: 0}
			} else {
				dist = maxZ
				maxZ += deltaZ
				current.Z += stepZ
				lastNormal = Vector3{X: 0, Y: float32(-stepZ), Z: 0}
			}
		} else {
			if maxY < maxZ {
				dist = maxY
				maxY += deltaY
				current.Y += stepY
				lastNormal = Vector3{X: 0, Y: 0, Z: float32(-stepY)}
			} else {
				dist = maxZ
				maxZ += deltaZ
				current.Z += stepZ
				lastNormal = Vector3{X: 0, Y: float32(-stepZ), Z: 0}
			}
		}
	}

	return false, DFCoord{}, Vector3{}
}
