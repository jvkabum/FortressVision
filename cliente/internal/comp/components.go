package comp

import (
)

// Position armazena a localização 3D no mundo Raylib
type Position struct {
	X, Y, Z float32
}

// Rotation armazena a rotação no eixo Y
type Rotation struct {
	Angle float32
}

// Scale armazena a escala do objeto
type Scale struct {
	Value float32
}

// Renderable contém os metadados para desenho
type Renderable struct {
	ModelName   string
	TextureName string
	Color       [4]uint8
	IsRamp      bool
}

// ChunkInfo associa a entidade a um chunk específico do DF para limpeza
type ChunkInfo struct {
	X, Y, Z int32
}

// Metadata armazena informações extras do tile (opcional por enquanto)
type Metadata struct {
	TileType int32
}
