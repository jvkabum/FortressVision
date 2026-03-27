package core

import (
	"kaijuengine.com/bootstrap"
	"kaijuengine.com/engine"
	"kaijuengine.com/engine/assets"
	"log"
	"os"
	"path/filepath"
	"reflect"
)

const gameContentPath = `game_content`

type App struct {
	Host *engine.Host
}

func (App) PluginRegistry() []reflect.Type { return nil }

func (App) ContentDatabase() (assets.Database, error) {
	if _, err := os.Stat(gameContentPath); err != nil {
		os.MkdirAll(gameContentPath, os.ModePerm)
	}
	
	// Sincroniza ativamente os assets core da engine
	enginePath := filepath.Join("..", "..", "kaiju")
	if err := SyncCoreAssets(enginePath, gameContentPath); err != nil {
		log.Printf("⚠️ Erro na sincronização de assets (ignorado se em prod): %v", err)
	}
	
	return assets.NewFileDatabase(gameContentPath)
}

func (a *App) Launch(host *engine.Host) {
	a.Host = host
	log.Println("🏗️ [Core] Fundação inicializada com sucesso.")
	
	// Configurações de Estabilidade (Alicerce)
	host.SetFrameRateLimit(0)
}

func Start(app bootstrap.GameInterface) {
	SetupLogger()
	bootstrap.Main(app, nil)
}
