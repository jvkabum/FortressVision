package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config armazena as configurações do FortressVision.
type Config struct {
	// Janela
	WindowWidth  int32  `json:"window_width"`
	WindowHeight int32  `json:"window_height"`
	WindowTitle  string `json:"window_title"`
	Fullscreen   bool   `json:"fullscreen"`
	TargetFPS    int32  `json:"target_fps"`

	// DFHack (Usado pelo Servidor)
	DFHackHost string `json:"dfhack_host"`
	DFHackPort int    `json:"dfhack_port"`

	// FortressVision Server (Usado pelo Cliente)
	ServerURL string `json:"server_url"`

	// Renderização
	DrawDistance  int32   `json:"draw_distance"`
	ViewLevels    int32   `json:"view_levels"`
	MesherThreads int     `json:"mesher_threads"`
	FOV           float32 `json:"fov"`
	DrawRangeDown int32   `json:"draw_range_down"` // Níveis Z abaixo da câmera
	DrawRangeUp   int32   `json:"draw_range_up"`   // Níveis Z acima da câmera
	DrawRangeSide int32   `json:"draw_range_side"` // Raio horizontal em blocos de 16 tiles

	// Câmera
	CameraSpeed       float32 `json:"camera_speed"`
	CameraSensitivity float32 `json:"camera_sensitivity"`
	ZoomSpeed         float32 `json:"zoom_speed"`

	// Debug
	ShowDebugInfo bool `json:"show_debug_info"`
	ShowGrid      bool `json:"show_grid"`
	WireframeMode bool `json:"wireframe_mode"`
}

// DefaultConfig retorna a configuração padrão.
func DefaultConfig() *Config {
	return &Config{
		WindowWidth:  1280,
		WindowHeight: 720,
		WindowTitle:  "FortressVision",
		Fullscreen:   false,
		TargetFPS:    60,

		DFHackHost: "localhost",
		DFHackPort: 5000,

		ServerURL: "ws://127.0.0.1:8080/ws",

		DrawDistance:  10,
		ViewLevels:    5,
		MesherThreads: 4,
		FOV:           60.0,
		DrawRangeDown: 5,
		DrawRangeUp:   1,
		DrawRangeSide: 4,

		CameraSpeed:       10.0,
		CameraSensitivity: 0.3,
		ZoomSpeed:         5.0,

		ShowDebugInfo: true,
		ShowGrid:      false,
		WireframeMode: false,
	}
}

// configPath retorna o caminho do arquivo de configuração.
func configPath() string {
	execDir, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(execDir), "config.json")
}

// Load carrega as configurações de um arquivo JSON.
// Se o arquivo não existir, retorna as configurações padrão.
func Load() *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}

	return cfg
}

// Save salva as configurações em um arquivo JSON.
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}
