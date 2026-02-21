package mapdata

import (
	"FortressVision/pkg/dfproto"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// MaterialStore gerencia as cores e propriedades visuais dos materiais.
type MaterialStore struct {
	// Cache de cores por par de material (MatType, MatIndex)
	Colors map[dfproto.MatPair]rl.Color

	// Mapeamento de texturas carregadas no Renderer (armazenamos apenas os nomes/IDs aqui)
	TextureMap map[dfproto.TiletypeMaterial]string
}

func NewMaterialStore() *MaterialStore {
	return &MaterialStore{
		Colors:     make(map[dfproto.MatPair]rl.Color),
		TextureMap: make(map[dfproto.TiletypeMaterial]string),
	}
}

// GetTextureName retorna o nome do arquivo de textura para uma categoria de material.
func (s *MaterialStore) GetTextureName(mat dfproto.TiletypeMaterial) string {
	switch mat {
	case dfproto.TilematStone, dfproto.TilematHBM, dfproto.TilematFeatureStone:
		return "stone"
	case dfproto.TilematMineral:
		return "ore"
	case dfproto.TilematFrozenLiquid:
		return "marble" // Gelo parece mármore
	case dfproto.TilematSoil, dfproto.TilematGrass, dfproto.TilematGrassDark, dfproto.TilematGrassDead, dfproto.TilematGrassDry:
		return "grass"
	case dfproto.TilematTreeMaterial, dfproto.TilematPlant, dfproto.TilematMushroom:
		return "wood"
	case dfproto.TilematConstruction:
		return "marble"
	case dfproto.TilematLava, dfproto.TilematMagma:
		return "ore" // Magma rock
	}
	// Se for uma categoria de gema (baseado no rfr.go, gemas são maiores que Stone)
	if mat >= dfproto.TilematStone && mat <= dfproto.TilematMineral {
		return "gem"
	}
	return ""
}

// GetTileColor retorna a cor para um tile específico.
func (s *MaterialStore) GetTileColor(tile *Tile) rl.Color {
	// 1. Tentar cor do material específico (se houver no cache)
	if color, ok := s.Colors[tile.Material]; ok {
		return color
	}

	// 2. Fallback por categoria de material do Tiletype
	// Precisamos saber a categoria do material do tiletype dele
	// (Isso exige que o Tile tenha acesso aos Tiletypes)

	switch tile.MaterialCategory() {
	case dfproto.TilematStone:
		return rl.NewColor(120, 120, 120, 255)
	case dfproto.TilematSoil:
		return rl.NewColor(100, 70, 45, 255) // Brownish
	case dfproto.TilematGrass, dfproto.TilematGrassDark, dfproto.TilematGrassDead, dfproto.TilematGrassDry:
		return rl.NewColor(60, 100, 40, 255) // Greener
	case dfproto.TilematTreeMaterial, dfproto.TilematPlant, dfproto.TilematMushroom:
		return rl.NewColor(110, 80, 50, 255)
	case dfproto.TilematMineral:
		return rl.NewColor(180, 180, 180, 255)
	case dfproto.TilematLava, dfproto.TilematMagma:
		return rl.NewColor(220, 50, 0, 255)
	case dfproto.TilematFrozenLiquid:
		return rl.NewColor(180, 230, 255, 255)
	case dfproto.TilematHBM, dfproto.TilematConstruction:
		return rl.NewColor(190, 190, 210, 255)
	}

	// Se não for nada conhecido, mas for parede, assume pedra
	if tile.IsWall() {
		return rl.NewColor(128, 128, 128, 255)
	}

	return rl.NewColor(150, 150, 150, 255) // Default grey
}

// UpdateMaterials popula o cache com as cores reais vindas do DFHack.
func (s *MaterialStore) UpdateMaterials(list *dfproto.MaterialList) {
	for _, mat := range list.MaterialList {
		s.Colors[mat.MatPair] = rl.NewColor(
			uint8(mat.StateColor.Red),
			uint8(mat.StateColor.Green),
			uint8(mat.StateColor.Blue),
			255,
		)
	}
}
