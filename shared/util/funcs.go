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

// Abs retorna o valor absoluto de um int32.
func Abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

// Max retorna o maior de dois int32.
func Max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// Min retorna o menor de dois int32.
func Min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
