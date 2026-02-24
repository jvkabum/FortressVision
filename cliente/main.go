package main

import (
	"flag"
	"log"
	"os"
	"runtime"

	"FortressVision/cliente/internal/app"
	"FortressVision/shared/config"
)

func main() {
	// IMPORTANTE para estabilidade no Windows: Raylib/OpenGL exige rodar na thread principal do SO
	runtime.LockOSThread()

	// Flags de linha de comando
	serverURL := flag.String("server", "", "URL do Servidor FortressVision (padrão: ws://localhost:8080/ws)")
	fullscreen := flag.Bool("fullscreen", false, "Iniciar em tela cheia")
	debug := flag.Bool("debug", false, "Mostrar informações de debug")
	width := flag.Int("width", 0, "Largura da janela")
	height := flag.Int("height", 0, "Altura da janela")
	flag.Parse()

	// Configurar Log em Arquivo
	f, err := os.OpenFile("debug_fv.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
		log.Println("--- INICIANDO FORTRESS VISION ---")
	}

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("╔══════════════════════════════════════╗")
	log.Println("║       FortressVision v0.1.0          ║")
	log.Println("║   Visualizador 3D para Dwarf Fortress║")
	log.Println("╚══════════════════════════════════════╝")

	// Carregar configurações
	cfg := config.Load()

	// Aplicar flags de linha de comando (sobrescrevem o config salvo)
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *fullscreen {
		cfg.Fullscreen = true
	}
	if *debug {
		cfg.ShowDebugInfo = true
	}
	if *width > 0 {
		cfg.WindowWidth = int32(*width)
	}
	if *height > 0 {
		cfg.WindowHeight = int32(*height)
	}

	// Criar e rodar a aplicação
	application := app.New(cfg)
	application.Run()
}
