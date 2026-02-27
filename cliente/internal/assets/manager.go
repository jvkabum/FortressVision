package assets

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// --- Estruturas JSON ---

// Position define coordenadas X,Y dentro de um building multi-tile
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// TextureSet agrupa os mapas PBR de um modelo
type TextureSet struct {
	Pattern      string `json:"pattern,omitempty"`
	Normal       string `json:"normal,omitempty"`
	Occlusion    string `json:"occlusion,omitempty"`
	Alpha        string `json:"alpha,omitempty"`
	Specular     string `json:"specular,omitempty"`
	Metallic     string `json:"metallic,omitempty"`
	Illumination string `json:"illumination,omitempty"`
}

// SubObject define uma variação ou peça posicional de um modelo
type SubObject struct {
	File           string      `json:"file"`
	Rotation       string      `json:"rotation,omitempty"`
	MaterialToken  string      `json:"materialToken,omitempty"`
	MaterialTokens []string    `json:"materialTokens,omitempty"`
	RampIndex      *int        `json:"rampIndex,omitempty"`
	Coverage       string      `json:"coverage,omitempty"`
	Position       *Position   `json:"position,omitempty"`
	Textures       *TextureSet `json:"textures,omitempty"`
}

// MeshEntry define a regra mestre que conecta Token do jogo ao Modelo 3D
type MeshEntry struct {
	File       string      `json:"file"`
	Rotation   string      `json:"rotation,omitempty"`
	Tokens     []string    `json:"tokens"`
	SubObjects []SubObject `json:"subObjects,omitempty"`
	Textures   *TextureSet `json:"textures,omitempty"`
	Comment    string      `json:"comment,omitempty"`
}

// TileMeshConfig é o root do tile_meshes.json
type TileMeshConfig struct {
	TileMeshes []MeshEntry `json:"tileMeshes"`
}

// BuildingMeshConfig é o root do building_meshes.json
type BuildingMeshConfig struct {
	BuildingMeshes []MeshEntry `json:"buildingMeshes"`
}

// NamedModelsConfig é o root do models.json
type NamedModelsConfig struct {
	NamedModels map[string]string `json:"named_models"`
}

// --- Manager ---

// Manager é a estrutura central em memória que responde às consultas do Mesher
type Manager struct {
	tileMeshes     []MeshEntry
	buildingMeshes []MeshEntry
	namedModels    map[string]string
}

// NewManager cria e carrega o gerenciador de assets a partir dos JSONs configurados
func NewManager(configDir string) (*Manager, error) {
	m := &Manager{}

	// Carregar tile meshes
	tileData, err := os.ReadFile(configDir + "/tile_meshes.json")
	if err != nil {
		return nil, fmt.Errorf("falha ao ler tile_meshes.json: %w", err)
	}
	var tileConf TileMeshConfig
	if err := json.Unmarshal(tileData, &tileConf); err != nil {
		return nil, fmt.Errorf("falha ao parsear tile_meshes.json: %w", err)
	}
	m.tileMeshes = tileConf.TileMeshes

	// Carregar building meshes
	buildData, err := os.ReadFile(configDir + "/building_meshes.json")
	if err != nil {
		return nil, fmt.Errorf("falha ao ler building_meshes.json: %w", err)
	}
	var buildConf BuildingMeshConfig
	if err := json.Unmarshal(buildData, &buildConf); err != nil {
		return nil, fmt.Errorf("falha ao parsear building_meshes.json: %w", err)
	}
	m.buildingMeshes = buildConf.BuildingMeshes

	// Carregar named models (essentials)
	namedData, err := os.ReadFile(configDir + "/models.json")
	if err != nil {
		// Fallback silencioso se o arquivo não existir (opcional)
		// return nil, fmt.Errorf("falha ao ler models.json: %w", err)
		m.namedModels = make(map[string]string)
	} else {
		var namedConf NamedModelsConfig
		if err := json.Unmarshal(namedData, &namedConf); err != nil {
			return nil, fmt.Errorf("falha ao parsear models.json: %w", err)
		}
		m.namedModels = namedConf.NamedModels
	}

	return m, nil
}

// --- Wildcard Matching ---

// matchToken compara um token de consulta contra um padrão com suporte a wildcards (*)
// Formato do token: "SHAPE:SPECIAL:MATERIAL:VARIANT:DIRECTION"
// O wildcard '*' em qualquer segmento aceita qualquer valor
func matchToken(pattern, query string) bool {
	patParts := strings.Split(pattern, ":")
	queryParts := strings.Split(query, ":")

	// Se o padrão for apenas "*", aceita tudo
	if pattern == "*" {
		return true
	}

	// Se os tamanhos divergem, não pode casar
	if len(patParts) != len(queryParts) {
		return false
	}

	for i := range patParts {
		if patParts[i] == "*" {
			continue // wildcard aceita qualquer valor
		}
		if patParts[i] != queryParts[i] {
			return false
		}
	}
	return true
}

// --- Consultas Públicas ---

// GetTileMesh retorna o MeshEntry mais específico para um token de tile do DF
// Retorna nil se nenhum padrão casar
func (m *Manager) GetTileMesh(token string) *MeshEntry {
	var bestMatch *MeshEntry
	bestScore := -1

	for i := range m.tileMeshes {
		entry := &m.tileMeshes[i]
		for _, pat := range entry.Tokens {
			if matchToken(pat, token) {
				score := specificityScore(pat)
				if score > bestScore {
					bestScore = score
					bestMatch = entry
				}
			}
		}
	}
	return bestMatch
}

// GetBuildingMesh retorna o MeshEntry para um token de building do DF
// Retorna nil se nenhum padrão casar
func (m *Manager) GetBuildingMesh(token string) *MeshEntry {
	var bestMatch *MeshEntry
	bestScore := -1

	for i := range m.buildingMeshes {
		entry := &m.buildingMeshes[i]
		for _, pat := range entry.Tokens {
			if matchToken(pat, token) {
				score := specificityScore(pat)
				if score > bestScore {
					bestScore = score
					bestMatch = entry
				}
			}
		}
	}
	return bestMatch
}

// GetSubObjectForRamp retorna o SubObject correto para um índice de rampa
func (m *Manager) GetSubObjectForRamp(entry *MeshEntry, rampIdx int) *SubObject {
	if entry == nil {
		return nil
	}
	for i := range entry.SubObjects {
		if entry.SubObjects[i].RampIndex != nil && *entry.SubObjects[i].RampIndex == rampIdx {
			return &entry.SubObjects[i]
		}
	}
	return nil
}

// GetSubObjectForMaterial retorna o SubObject que casa com um material específico
func (m *Manager) GetSubObjectForMaterial(entry *MeshEntry, materialToken string) *SubObject {
	if entry == nil {
		return nil
	}
	for i := range entry.SubObjects {
		sub := &entry.SubObjects[i]
		// Checar campo singular
		if sub.MaterialToken != "" && matchToken(sub.MaterialToken, materialToken) {
			return sub
		}
		// Checar lista de materiais
		for _, mt := range sub.MaterialTokens {
			if matchToken(mt, materialToken) {
				return sub
			}
		}
	}
	return nil
}

// GetAllTileMeshes retorna todas as entradas de tile meshes carregadas
func (m *Manager) GetAllTileMeshes() []MeshEntry {
	return m.tileMeshes
}

// GetAllBuildingMeshes retorna todas as entradas de building meshes carregadas
func (m *Manager) GetAllBuildingMeshes() []MeshEntry {
	return m.buildingMeshes
}

// GetNamedModels retorna o mapa de modelos essenciais
func (m *Manager) GetNamedModels() map[string]string {
	return m.namedModels
}

// specificityScore calcula a "especificidade" de um padrão
// Quanto mais segmentos NÃO são wildcard, mais específico é
func specificityScore(pattern string) int {
	if pattern == "*" {
		return 0
	}
	parts := strings.Split(pattern, ":")
	score := 0
	for _, p := range parts {
		if p != "*" {
			score++
		}
	}
	return score
}
