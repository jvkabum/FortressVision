package util

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Ray representa um raio no espaço 3D (Origem e Direção)
type Ray struct {
	Origin    rl.Vector3
	Direction rl.Vector3
}

// Vector3 é um alias para rl.Vector3 para conveniência
type Vector3 = rl.Vector3

// DFCoord representa uma coordenada no espaço do Dwarf Fortress.
// X = leste/oeste, Y = norte/sul, Z = nível vertical
type DFCoord struct {
	X, Y, Z int32
}

// NewDFCoord cria uma nova coordenada DF.
func NewDFCoord(x, y, z int32) DFCoord {
	return DFCoord{X: x, Y: y, Z: z}
}

// Add soma duas coordenadas.
func (c DFCoord) Add(other DFCoord) DFCoord {
	return DFCoord{
		X: c.X + other.X,
		Y: c.Y + other.Y,
		Z: c.Z + other.Z,
	}
}

// Sub subtrai duas coordenadas.
func (c DFCoord) Sub(other DFCoord) DFCoord {
	return DFCoord{
		X: c.X - other.X,
		Y: c.Y - other.Y,
		Z: c.Z - other.Z,
	}
}

// Equals verifica igualdade entre coordenadas.
func (c DFCoord) Equals(other DFCoord) bool {
	return c.X == other.X && c.Y == other.Y && c.Z == other.Z
}

// String retorna a representação em string da coordenada.
func (c DFCoord) String() string {
	return fmt.Sprintf("(%d, %d, %d)", c.X, c.Y, c.Z)
}

// BlockSize é o tamanho de um bloco do mapa no DF (16x16x1).
const BlockSize = 16

// BlockCoord retorna a coordenada do bloco que contém esta coordenada.
func (c DFCoord) BlockCoord() DFCoord {
	return DFCoord{
		X: int32(math.Floor(float64(c.X)/float64(BlockSize))) * BlockSize,
		Y: int32(math.Floor(float64(c.Y)/float64(BlockSize))) * BlockSize,
		Z: c.Z,
	}
}

// LocalCoord retorna a coordenada local dentro do bloco (0-15, 0-15).
func (c DFCoord) LocalCoord() DFCoord {
	bc := c.BlockCoord()
	return DFCoord{
		X: c.X - bc.X,
		Y: c.Y - bc.Y,
		Z: 0,
	}
}

// GameScale controla a escala de conversão DF → 3D.
const GameScale float32 = 1.0

// DFToWorldPos converte uma coordenada DF para posição 3D no mundo.
// No DF: X = leste, Y = sul (invertido), Z = cima
// No 3D: X = leste, Y = cima (Z do DF), Z = sul (Y do DF invertido)
func DFToWorldPos(coord DFCoord) rl.Vector3 {
	return rl.Vector3{
		X: float32(coord.X) * GameScale,
		Y: float32(coord.Z) * GameScale,
		Z: float32(-coord.Y) * GameScale, // Y do DF é invertido
	}
}

// DFToWorldCenter converte para o centro do tile no mundo 3D.
func DFToWorldCenter(coord DFCoord) rl.Vector3 {
	pos := DFToWorldPos(coord)
	pos.X += GameScale * 0.5
	pos.Z -= GameScale * 0.5
	return pos
}

// FloorHeight é a altura visual de um piso (0.1 por padrão no Armok)
const FloorHeight float32 = 0.1

// DFToWorldBottomCorner retorna o canto inferior esquerdo do tile (espaço 3D).
func DFToWorldBottomCorner(coord DFCoord) rl.Vector3 {
	pos := DFToWorldPos(coord)
	// Como DFToWorldPos já retorna o canto "origin" do tile,
	// e nosso sistema já inverte o Y, o "BottomCorner"
	// no espaço 3D depende se o DFToWorldPos é o centro ou o canto.
	// No FortressVision V1, DFToWorldPos é o canto (X*Scale, Z*Scale, -Y*Scale).
	return pos
}

// Between verifica se um valor está entre um limite inferior e superior.
func Between(lower, t, upper float32) bool {
	return t >= lower && t <= upper
}
func WorldToDFCoord(pos rl.Vector3) DFCoord {
	return DFCoord{
		X: int32(math.Floor(float64(pos.X / GameScale))),
		Y: int32(math.Floor(float64(-pos.Z / GameScale))),
		Z: int32(math.Floor(float64(pos.Y / GameScale))),
	}
}

// Directions representa as direções cardinais e diagonais.
type Directions uint16

const (
	DirNone      Directions = 0
	DirNorthWest Directions = 1 << iota
	DirNorth
	DirNorthEast
	DirEast
	DirSouthEast
	DirSouth
	DirSouthWest
	DirWest
	DirUp
	DirDown
	DirAll Directions = 0x0FFF
)

// Has verifica se uma direção está ativa.
func (d Directions) Has(dir Directions) bool {
	return d&dir != 0
}

// DirOffsets mapeia direções para offsets de coordenada.
var DirOffsets = map[Directions]DFCoord{
	DirNorth:     {X: 0, Y: -1, Z: 0},
	DirSouth:     {X: 0, Y: 1, Z: 0},
	DirEast:      {X: 1, Y: 0, Z: 0},
	DirWest:      {X: -1, Y: 0, Z: 0},
	DirNorthEast: {X: 1, Y: -1, Z: 0},
	DirNorthWest: {X: -1, Y: -1, Z: 0},
	DirSouthEast: {X: 1, Y: 1, Z: 0},
	DirSouthWest: {X: -1, Y: 1, Z: 0},
	DirUp:        {X: 0, Y: 0, Z: 1},
	DirDown:      {X: 0, Y: 0, Z: -1},
}

// AddDir retorna uma nova coordenada deslocada na direção especificada.
func (c DFCoord) AddDir(dir Directions) DFCoord {
	return c.Add(DirOffsets[dir])
}

// Helpers para direções rápidas
func (c DFCoord) Up() DFCoord    { return c.AddDir(DirUp) }
func (c DFCoord) Down() DFCoord  { return c.AddDir(DirDown) }
func (c DFCoord) North() DFCoord { return c.AddDir(DirNorth) }
func (c DFCoord) South() DFCoord { return c.AddDir(DirSouth) }
func (c DFCoord) East() DFCoord  { return c.AddDir(DirEast) }
func (c DFCoord) West() DFCoord  { return c.AddDir(DirWest) }
