package render

import (
	"fmt"
	"log"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (r *Renderer) loadTextures() {
	// Texturas de blocos/terreno
	blocks := []string{"stone", "grass", "wood", "marble", "ore", "plant"}
	for _, name := range blocks {
		path := fmt.Sprintf("assets/textures/blocks/%s.png", name)
		r.loadSingleTexture(name, path)
	}

	// Texturas de itens
	items := []string{"gem"}
	for _, name := range items {
		path := fmt.Sprintf("assets/textures/items/%s.png", name)
		r.loadSingleTexture(name, path)
	}
}

func (r *Renderer) loadSingleTexture(name, path string) {
	tex := rl.LoadTexture(path)
	if tex.ID != 0 {
		rl.GenTextureMipmaps(&tex)
		rl.SetTextureFilter(tex, rl.FilterTrilinear)
		rl.SetTextureWrap(tex, rl.WrapRepeat)
		r.Textures[name] = tex
		log.Printf("[Renderer] Textura carregada: %s", path)
	} else {
		log.Printf("[Renderer] FALHA ao carregar textura: %s", path)
	}
}

func (r *Renderer) loadModels() {
	// Carregar modelos nomeados (essenciais) do Asset Manager (models.json)
	if r.AssetMgr != nil {
		for name, path := range r.AssetMgr.GetNamedModels() {
			r.loadSingleModel(name, path)
		}
	} else {
		// Fallback de modelos essenciais (sempre tenta carregar estes se o Manager falhar)
		models := []struct {
			name string
			path string
		}{
			{"shrub", "assets/models/environment/Foliage_New.obj"},
			{"tree_trunk", "assets/models/environment/TreeTrunkPillar.obj"},
			{"tree_branches", "assets/models/environment/TreeBranches.obj"},
			{"tree_twigs", "assets/models/environment/TreeTwigs.obj"},
			{"branches", "assets/models/environment/Branches.obj"},
			{"mushroom", "assets/models/environment/Foliage_Small_1.obj"},
		}
		for _, m := range models {
			r.loadSingleModel(m.name, m.path)
		}
	}

	// Carregar modelos adicionais baseados em tokens
	if r.AssetMgr != nil {
		loaded := make(map[string]bool)
		for _, name := range r.getModelNames() {
			loaded[name] = true
		}

		// Tile meshes do JSON
		for _, entry := range r.AssetMgr.GetAllTileMeshes() {
			if entry.File == "" || entry.File == "NONE" {
				continue
			}
			path := "assets/models/" + entry.File
			if !loaded[path] {
				r.loadSingleModel(entry.File, path)
				loaded[path] = true
			}
			// SubObjects
			for _, sub := range entry.SubObjects {
				if sub.File == "" || sub.File == "NONE" {
					continue
				}
				subPath := "assets/models/" + sub.File
				if !loaded[subPath] {
					r.loadSingleModel(sub.File, subPath)
					loaded[subPath] = true
				}
			}
		}

		// Building meshes do JSON
		for _, entry := range r.AssetMgr.GetAllBuildingMeshes() {
			if entry.File == "" || entry.File == "NONE" {
				continue
			}
			path := "assets/models/" + entry.File
			if !loaded[path] {
				r.loadSingleModel(entry.File, path)
				loaded[path] = true
			}
			for _, sub := range entry.SubObjects {
				if sub.File == "" || sub.File == "NONE" {
					continue
				}
				subPath := "assets/models/" + sub.File
				if !loaded[subPath] {
					r.loadSingleModel(sub.File, subPath)
					loaded[subPath] = true
				}
			}
		}

		log.Printf("[Renderer] Total de modelos 3D carregados via JSON: %d", len(r.Models3D))
	}
}

func (r *Renderer) loadSingleModel(name, path string) {
	model := rl.LoadModel(path)
	if model.MeshCount > 0 {
		// Usamos o 'name' (que vem do campo 'File' do JSON ou do manual) como chave
		r.Models3D[name] = model
		log.Printf("[Renderer] Modelo carregado: %s (Key: %s)", path, name)
	} else {
		log.Printf("[Renderer] FALHA ao carregar modelo: %s", path)
	}
}

func (r *Renderer) getModelNames() []string {
	names := make([]string, 0, len(r.Models3D))
	for k := range r.Models3D {
		names = append(names, k)
	}
	return names
}
