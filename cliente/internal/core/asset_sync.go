package core

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// SyncCoreAssets verifica e provisiona automaticamente os arquivos essenciais
// da Kaiju Engine achatando-os diretamente para a raiz da pasta game_content
func SyncCoreAssets(engineSourcePath string, gameContentDest string) error {
	log.Println("🤖 [AssetSync] Inspecionando o provisionamento de ativos em formato Flat...")

	embeddedPath := filepath.Join(engineSourcePath, "src", "editor", "editor_embedded_content", "editor_content")
	os.MkdirAll(gameContentDest, os.ModePerm)

	skipRootFolders := []string{"editor", "meshes"}

	filepath.Walk(embeddedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			// Ignora certas pastas da raiz do editor
			if filepath.Dir(path) == embeddedPath {
				for _, skip := range skipRootFolders {
					if info.Name() == skip {
						return filepath.SkipDir
					}
				}
			}
			// Ignora o código-fonte dos shaders (o binário Spv já é copiado)
			if info.Name() == "src" && strings.HasSuffix(filepath.Dir(path), "renderer") {
				return filepath.SkipDir
			}
			return nil
		}

		// ACHATAMENTO (Flattening): todo arquivo detectado cai direto na BASE do game_content
		destFilePath := filepath.Join(gameContentDest, info.Name())

		// Fallback inteligente: só copia se você (o Dev) não tiver sobrescrito e o arquivo estiver em falta
		if _, err := os.Stat(destFilePath); os.IsNotExist(err) {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				os.WriteFile(destFilePath, data, os.ModePerm)
			}
		}
		
		return nil
	})

	log.Println("✅ [AssetSync] O ecossistema de renderização da Kaiju foi injetado (Flat-Root).")
	return nil
}
