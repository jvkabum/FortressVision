package mapdata

import (
	"FortressVision/shared/pkg/dfproto"
	"fmt"
	"log"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
	"gorm.io/gorm"
)

// MaterialStore gerencia as cores e propriedades visuais dos materiais.
type MaterialStore struct {
	mu sync.RWMutex

	// Cache de cores por par de material (MatType, MatIndex)
	Colors map[dfproto.MatPair]rl.Color

	// Cache de nomes legíveis (ex: "Granite", "Iron Ore")
	Names map[dfproto.MatPair]string

	// Cache de IDs internos/tokens (ex: "STONE:GRANITE")
	Tokens map[dfproto.MatPair]string

	// Mapeamento de texturas carregadas no Renderer (armazenamos apenas os nomes/IDs aqui)
	TextureMap map[dfproto.TiletypeMaterial]string

	// Referência ao banco de dados para persistência
	DB *gorm.DB
}

func NewMaterialStore() *MaterialStore {
	return &MaterialStore{
		Colors:     make(map[dfproto.MatPair]rl.Color),
		Names:      make(map[dfproto.MatPair]string),
		Tokens:     make(map[dfproto.MatPair]string),
		TextureMap: make(map[dfproto.TiletypeMaterial]string),
	}
}

// GetTextureName retorna o nome do arquivo de textura para uma categoria de material.
func (s *MaterialStore) GetTextureName(mat dfproto.TiletypeMaterial) string {
	switch mat {
	case dfproto.TilematStone, dfproto.TilematHFS, dfproto.TilematFeature:
		return "stone"
	case dfproto.TilematMineral:
		return "ore"
	case dfproto.TilematFrozenLiquid:
		return "marble" // Gelo parece mármore
	case dfproto.TilematSoil, dfproto.TilematGrassLight, dfproto.TilematGrassDark, dfproto.TilematGrassDead, dfproto.TilematGrassDry:
		return "grass"
	case dfproto.TilematTreeMaterial, dfproto.TilematPlant, dfproto.TilematMushroom:
		return "wood"
	case dfproto.TilematConstruction:
		return "marble"
	case dfproto.TilematLavaStone, dfproto.TilematMagma:
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Tentar cor do material específico (se houver no cache enviada pelo DFHack)
	if color, ok := s.Colors[tile.Material]; ok {
		return color
	}

	// 2. Fallback baseado no MaterialCategory e cores padrão do DF
	// Isso usa a tabela DFColorList portada do Armok Vision.
	var colorToken string
	switch tile.MaterialCategory() {
	case dfproto.TilematStone:
		colorToken = "GRAY"
	case dfproto.TilematSoil:
		colorToken = "DARK_TAN"
	case dfproto.TilematGrassLight, dfproto.TilematGrassDark, dfproto.TilematGrassDead, dfproto.TilematGrassDry:
		colorToken = "GREEN"
	case dfproto.TilematTreeMaterial, dfproto.TilematPlant, dfproto.TilematMushroom:
		colorToken = "BROWN"
	case dfproto.TilematMineral:
		colorToken = "SILVER"
	case dfproto.TilematLavaStone, dfproto.TilematMagma:
		colorToken = "RED"
	case dfproto.TilematFrozenLiquid:
		colorToken = "PALE_BLUE"
	case dfproto.TilematHFS, dfproto.TilematConstruction:
		colorToken = "WHITE"
	default:
		colorToken = "GRAY"
	}

	r, g, b, ok := GetDFColor(colorToken)
	if ok {
		return rl.NewColor(r, g, b, 255)
	}

	return rl.NewColor(150, 150, 150, 255) // Fallback absoluto
}

// LoadFromDB carrega todos os materiais salvos no SQLite para o cache.
func (s *MaterialStore) LoadFromDB() error {
	if s.DB == nil {
		return nil
	}

	var models []MaterialModel
	if err := s.DB.Find(&models).Error; err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range models {
		pair := dfproto.MatPair{
			MatType:  m.MatType,
			MatIndex: m.MatIndex,
		}
		s.Colors[pair] = rl.NewColor(m.R, m.G, m.B, 255)
	}
	log.Printf("[MaterialStore] %d materiais carregados do SQLite.", len(models))
	return nil
}

// UpdateMaterials popula o cache com as cores reais vindas do DFHack e persiste no banco.
func (s *MaterialStore) UpdateMaterials(list *dfproto.MaterialList) {
	if list == nil || len(list.MaterialList) == 0 {
		return
	}

	s.mu.Lock()
	var models []MaterialModel
	for _, mat := range list.MaterialList {
		pair := mat.MatPair
		color := rl.NewColor(
			uint8(mat.StateColor.Red),
			uint8(mat.StateColor.Green),
			uint8(mat.StateColor.Blue),
			255,
		)
		s.Colors[pair] = color
		s.Names[pair] = mat.Name
		s.Tokens[pair] = mat.ID

		if s.DB != nil {
			models = append(models, MaterialModel{
				MatType:  pair.MatType,
				MatIndex: pair.MatIndex,
				R:        color.R,
				G:        color.G,
				B:        color.B,
			})
		}
	}
	s.mu.Unlock()

	// Salva no banco de dados em lotes (Batch Insert/Upsert)
	if s.DB != nil && len(models) > 0 {
		log.Printf("[MaterialStore] Salvando %d novos materiais no banco...", len(models))
		err := s.DB.Save(&models).Error
		if err != nil {
			log.Printf("[MaterialStore] Erro ao persistir materiais: %v", err)
		}
	}
}

// GetMaterialName retorna o nome legível do material ou seu token como fallback.
func (s *MaterialStore) GetMaterialName(pair dfproto.MatPair) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name, ok := s.Names[pair]; ok && name != "" {
		return name
	}

	if token, ok := s.Tokens[pair]; ok && token != "" {
		return token
	}

	return fmt.Sprintf("Desconhecido (%d:%d)", pair.MatType, pair.MatIndex)
}
