package util

import rl "github.com/gen2brain/raylib-go/raylib"

// Lerp realiza interpolação linear entre dois floats.
func Lerp(start, end, amount float32) float32 {
	return start + amount*(end-start)
}

// DistSq retorna a distância quadrada entre dois vetores 3D.
func DistSq(v1, v2 rl.Vector3) float32 {
	dx := v1.X - v2.X
	dy := v1.Y - v2.Y
	dz := v1.Z - v2.Z
	return dx*dx + dy*dy + dz*dz
}
