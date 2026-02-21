package util

// Lerp realiza interpolação linear entre dois floats.
func Lerp(start, end, amount float32) float32 {
	return start + amount*(end-start)
}
