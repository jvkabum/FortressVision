package app

import (
	"log"

	"FortressVision/cliente/internal/camera"
	"FortressVision/cliente/internal/render"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// updateCamera atualiza a câmera baseado no input.
func (a *App) updateCamera() {
	dt := rl.GetFrameTime()

	// Processa input (WASD, Mouse, Zoom)
	if a.Cam.HandleInput(dt) {
		a.lastManualMove = int64(rl.GetTime() * 1000) // Converte segundos para ms
	}

	// Atualiza física/interpolação da câmera
	a.Cam.Update(dt)

	// Nível Z com Q/E (REPETIÇÃO CONTÍNUA AO SEGURAR)
	zRepeatDelay := 0.08 // Velocidade de descida/subida rápida
	if rl.IsKeyPressed(rl.KeyE) || (rl.IsKeyDown(rl.KeyE) && rl.GetTime()-a.lastZKeyTime > zRepeatDelay) {
		a.mapCenter.Z++
		a.Cam.TargetLookAt.Y += util.GameScale
		a.lastZKeyTime = rl.GetTime()
		a.updateMap(true) // Instant Z-Sync (Phase 29)
	}
	if rl.IsKeyPressed(rl.KeyQ) || (rl.IsKeyDown(rl.KeyQ) && rl.GetTime()-a.lastZKeyTime > zRepeatDelay) {
		a.mapCenter.Z--
		a.Cam.TargetLookAt.Y -= util.GameScale
		a.lastZKeyTime = rl.GetTime()
		a.updateMap(true) // Instant Z-Sync (Phase 29)
	}

	// Alternar projeção com P
	if rl.IsKeyPressed(rl.KeyP) {
		if a.Cam.Mode == camera.ModePerspective {
			a.Cam.SetMode(camera.ModeOrthographic)
			log.Println("[Camera] Modo Ortográfico")
		} else {
			a.Cam.SetMode(camera.ModePerspective)
			log.Println("[Camera] Modo Perspectiva")
		}
	}
}

// updateInput processa entradas de teclado gerais.
func (a *App) updateInput() {
	// Toggle debug info
	if rl.IsKeyPressed(rl.KeyF3) {
		a.Config.ShowDebugInfo = !a.Config.ShowDebugInfo
	}

	// Pular Loading manualmente
	if a.Loading && rl.IsKeyPressed(rl.KeySpace) {
		log.Println("[App] Loading/Download pulado manualmente pelo usuário.")
		a.Loading = false
		a.FullScanActive = false
		// No modo C/S, o loading termina quando os primeiros chunks chegam
	}

	// Toggle grid
	if rl.IsKeyPressed(rl.KeyG) {
		a.Config.ShowGrid = !a.Config.ShowGrid
	}

	// Toggle wireframe
	if rl.IsKeyPressed(rl.KeyF4) {
		a.Config.WireframeMode = !a.Config.WireframeMode
	}

	// Toggle weather with F7
	if rl.IsKeyPressed(rl.KeyF7) {
		if a.renderer != nil && a.renderer.Weather != nil {
			newType := (int(a.renderer.Weather.Type) + 1) % 3
			a.renderer.Weather.Type = render.WeatherType(newType)
			log.Printf("[App] Clima alterado para: %v", a.renderer.Weather.Type)
		}
	}

	// Fullscreen toggle
	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleFullscreen()
	}

	// Inspecionar com Clique Direito (Fase 35)
	if rl.IsMouseButtonPressed(rl.MouseRightButton) {
		mousePos := rl.GetMousePosition()
		ray := rl.GetMouseRay(mousePos, a.Cam.RLCamera)
		coord, hit := a.renderer.GetRayCollision(ray)
		if hit {
			a.SelectedCoord = &coord
		} else {
			a.SelectedCoord = nil
		}
	}

	// ESC: Alternar Pausa/Menu (Fase 10)
	if rl.IsKeyPressed(rl.KeyEscape) {
		if a.State == StateViewing {
			a.State = StatePaused
			log.Println("[App] Jogo Pausado")
		} else if a.State == StatePaused {
			a.State = StateViewing
			log.Println("[App] Retomando Jogo")
		}
	}

	// Ajuste manual de Offset Z (Teclas [ e ])
	if rl.IsKeyPressed(rl.KeyLeftBracket) {
		a.ZOffset--
		log.Printf("[App] Offset Z ajustado para: %d", a.ZOffset)
	}
	if rl.IsKeyPressed(rl.KeyRightBracket) {
		a.ZOffset++
		log.Printf("[App] Offset Z ajustado para: %d", a.ZOffset)
	}
}
