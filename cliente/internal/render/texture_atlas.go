package render

import (
	"FortressVision/cliente/internal/assets/constants"
	"log"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// LeafAtlas gerencia texturas de folhas em um único atlas
type LeafAtlas struct {
	Texture  rl.Texture2D
	Regions  map[string]rl.Rectangle
	TileSize int32
}

func (r *Renderer) createLeafAtlas() {
	atlas := &LeafAtlas{
		Regions:  make(map[string]rl.Rectangle),
		TileSize: 16,
	}

	// Lista de texturas de folha para incluir no atlas
	leafTypes := []string{
		constants.TextureLeafOak,
		constants.TextureLeafPine,
		constants.TextureLeafMaple,
		constants.TextureLeafMushroom,
		constants.TextureLeafGeneric,
	}

	// Grid 4x2
	atlasWidth := atlas.TileSize * 4
	atlasHeight := atlas.TileSize * 2

	atlasImg := rl.GenImageColor(int(atlasWidth), int(atlasHeight), rl.Blank)

	for i, name := range leafTypes {
		x := int32(i%4) * atlas.TileSize
		y := int32(i/4) * atlas.TileSize

		var img *rl.Image
		if tex, ok := r.Textures[name]; ok && tex.ID != 0 {
			tempImg := rl.LoadImageFromTexture(tex)
			img = tempImg
		} else {
			img = r.generateFallbackLeafImage(name)
		}

		rl.ImageDraw(atlasImg, img, rl.NewRectangle(0, 0, float32(atlas.TileSize), float32(atlas.TileSize)), 
			rl.NewRectangle(float32(x), float32(y), float32(atlas.TileSize), float32(atlas.TileSize)), rl.White)

		rl.UnloadImage(img)

		atlas.Regions[name] = rl.NewRectangle(
			float32(x)/float32(atlasWidth),
			float32(y)/float32(atlasHeight),
			float32(atlas.TileSize)/float32(atlasWidth),
			float32(atlas.TileSize)/float32(atlasHeight),
		)
	}

	atlas.Texture = rl.LoadTextureFromImage(atlasImg)
	rl.SetTextureFilter(atlas.Texture, rl.FilterPoint) // Estilo Pixel Art
	rl.UnloadImage(atlasImg)

	r.LeafAtlas = atlas
	log.Printf("[Texture] Atlas de folhas criado: %dx%d", atlasWidth, atlasHeight)
}

func (r *Renderer) generateFallbackLeafImage(name string) *rl.Image {
	img := rl.GenImageColor(16, 16, rl.Green)
	
	switch name {
	case constants.TextureLeafOak:
		rl.ImageColorTint(img, rl.NewColor(80, 120, 40, 255))
	case constants.TextureLeafPine:
		rl.ImageColorTint(img, rl.NewColor(60, 100, 50, 255))
	case constants.TextureLeafMaple:
		rl.ImageColorTint(img, rl.NewColor(100, 80, 40, 255))
	case constants.TextureLeafMushroom:
		rl.ImageColorTint(img, rl.NewColor(200, 180, 140, 255))
	default:
		rl.ImageColorTint(img, rl.NewColor(70, 110, 45, 255))
	}
	
	// Padrão simples
	for i := int32(0); i < 16; i++ {
		rl.ImageDrawPixel(img, i, 8, rl.DarkGreen)
		rl.ImageDrawPixel(img, 8, i, rl.DarkGreen)
	}
	
	return img
}
