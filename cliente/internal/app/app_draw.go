package app

import (
	"fmt"
	"log"
	"runtime"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// draw renderiza a cena.
func (a *App) draw() {
	rl.BeginDrawing()
	rl.ClearBackground(rl.NewColor(30, 30, 40, 255))

	if a.Loading {
		a.drawLoadingScreen()
	} else {
		a.drawScene()
		a.drawHUD()

		if a.State == StatePaused {
			a.drawPauseMenu()
		}
	}

	rl.EndDrawing()
}

// drawScene renderiza a cena 3D.
func (a *App) drawScene() {
	rl.BeginMode3D(a.Cam.RLCamera)

	// Grid de referência
	if a.Config.ShowGrid {
		rl.DrawGrid(40, util.GameScale)
	}

	// Renderizar modelos do mapa real
	if a.renderer != nil {
		a.renderer.Draw(a.Cam.RLCamera, a.mapCenter.Z)

		// Desenhar destaque de seleção (Fase 35)
		if a.SelectedCoord != nil {
			a.renderer.DrawSelection(*a.SelectedCoord)
		}
	}

	rl.EndMode3D()
}

// drawHUD desenha a interface sobreposta.
func (a *App) drawHUD() {
	if !a.Config.ShowDebugInfo {
		return
	}

	// Fundo semi-transparente para o debug (Aumentado para Fase 9)
	width := int32(340)
	height := int32(240)
	x := int32(rl.GetScreenWidth()) - width - 10
	y := int32(10)

	rl.DrawRectangle(x, y, width, height, rl.NewColor(0, 0, 0, 180))
	rl.DrawRectangleLines(x, y, width, height, rl.NewColor(50, 50, 50, 255))

	// FPS
	fps := rl.GetFPS()
	fpsColor := rl.Green
	if fps < 30 {
		fpsColor = rl.Red
	} else if fps < 50 {
		fpsColor = rl.Yellow
	}
	rl.DrawText(fmt.Sprintf("FPS: %d", fps), x+10, y+10, 20, fpsColor)

	// Estado do Clima (Novo na Fase 8)
	weatherStr := "Dia Limpo"
	weatherColor := rl.SkyBlue
	if a.renderer != nil && a.renderer.Weather != nil {
		switch a.renderer.Weather.Type {
		case 1: // WeatherRain
			weatherStr = "Chuva"
			weatherColor = rl.Blue
		case 2: // WeatherSnow
			weatherStr = "Neve"
			weatherColor = rl.White
		}
	}
	rl.DrawText(weatherStr, x+215, y+10, 20, weatherColor)

	// Divisor
	rl.DrawLine(x+10, y+35, x+width-10, y+35, rl.NewColor(100, 100, 100, 100))

	// Informações de Localização
	rl.DrawText("LOCALIZAÇÃO", x+10, y+45, 12, rl.Gray)

	dfCoord := util.WorldToDFCoord(a.Cam.CurrentLookAt)
	dfCoord.Z = a.mapCenter.Z

	rl.DrawText(fmt.Sprintf("Coord DF: (%d, %d, %d)", dfCoord.X, dfCoord.Y, dfCoord.Z), x+10, y+60, 16, rl.White)

	elevation := dfCoord.Z - a.ZOffset
	syncStatus := "Offline"
	if a.netClient != nil && a.netClient.IsConnected() {
		syncStatus = "Conectado"
	}

	rl.DrawText(fmt.Sprintf("Elevação: %d (Offset:%d) [%s]", elevation, a.ZOffset, syncStatus), x+10, y+80, 14, rl.LightGray)

	// Divisor
	rl.DrawLine(x+10, y+100, x+width-10, y+100, rl.NewColor(100, 100, 100, 100))

	// Info do Mundo (Novo na Fase 9)
	worldStr := fmt.Sprintf("%d, %s %d - %s", a.WorldYear, a.WorldMonth, a.WorldDay, a.WorldSeason)
	if a.WorldName != "" {
		rl.DrawText(a.WorldName, x+10, y+110, 14, rl.Gold)
	}
	rl.DrawText(worldStr, x+10, y+125, 14, rl.LightGray)
	rl.DrawText(fmt.Sprintf("População: %d criaturas", a.WorldPopulation), x+10, y+140, 14, rl.LightGray)

	// Divisor
	rl.DrawLine(x+10, y+160, x+width-10, y+160, rl.NewColor(100, 100, 100, 100))

	// Atalhos Rápidos
	rl.DrawText("CONTROLES", x+10, y+170, 12, rl.Gray)
	rl.DrawText("Q/E: Nível Z | Scroll: Zoom | WASD: Mover", x+10, y+185, 14, rl.LightGray)

	// Painel de Inspeção (Fase 35)
	a.drawSelectedTileInfo()

	wireframeExtra := ""
	if a.Config.WireframeMode {
		wireframeExtra = " [WIREFRAME ON]"
	}
	rl.DrawText(fmt.Sprintf("F7: Clima | F11: Tela Cheia | F3: HUD%s", wireframeExtra), x+10, y+205, 14, rl.SkyBlue)

	// Título no canto inferior direito
	title := "FortressVision v0.1.0 - Alpha"
	titleWidth := rl.MeasureText(title, 18)
	rl.DrawText(title,
		int32(rl.GetScreenWidth())-titleWidth-20, int32(rl.GetScreenHeight())-30,
		18, rl.NewColor(200, 200, 200, 150))
}

func (a *App) drawSelectedTileInfo() {
	if a.SelectedCoord == nil {
		return
	}

	tile := a.mapStore.GetTile(*a.SelectedCoord)
	if tile == nil {
		return
	}

	// Painel de Detalhes (Lado Direito)
	width := int32(280)
	height := int32(180)
	x := int32(rl.GetScreenWidth()) - width - 10
	y := int32(260) // Abaixo do HUD principal (240 + 10 margem + 10 respiro)

	// Fundo semi-transparente
	rl.DrawRectangle(x, y, width, height, rl.NewColor(0, 0, 0, 200))
	rl.DrawRectangleLines(x, y, width, height, rl.NewColor(255, 215, 0, 255)) // Borda Dourada

	rl.DrawText("INSPEÇÃO DE BLOCO", x+15, y+15, 18, rl.Gold)
	rl.DrawLine(x+15, y+40, x+width-15, y+40, rl.NewColor(100, 100, 100, 255))

	// Informações do Tile
	rl.DrawText(fmt.Sprintf("Coord: %v", a.SelectedCoord.String()), x+15, y+50, 16, rl.White)
	rl.DrawText(fmt.Sprintf("TileType ID: %d", tile.TileType), x+15, y+70, 16, rl.LightGray)

	// Material real (Fase 36)
	matName := a.matStore.GetMaterialName(tile.Material)
	rl.DrawText(fmt.Sprintf("Material: %s", matName), x+15, y+90, 16, rl.White)

	// Lógica de categorias de material (Fase 37)
	matCat := "Desconhecido"
	cat := tile.MaterialCategory()
	switch cat {
	case dfproto.TilematStone:
		matCat = "Pedra"
	case dfproto.TilematSoil:
		matCat = "Solo"
	case dfproto.TilematGrassLight, dfproto.TilematGrassDark, dfproto.TilematGrassDry, dfproto.TilematGrassDead:
		matCat = "Grama"
	case dfproto.TilematTreeMaterial, dfproto.TilematPlant, dfproto.TilematMushroom:
		matCat = "Vegetação"
	case dfproto.TilematMineral:
		matCat = "Minério"
	case dfproto.TilematMagma:
		matCat = "Magma"
	case dfproto.TilematConstruction:
		matCat = "Construção"
	case dfproto.TilematFrozenLiquid:
		matCat = "Gelo"
	default:
		matCat = fmt.Sprintf("Outro (%d)", cat)
	}
	rl.DrawText(fmt.Sprintf("Categoria: %s", matCat), x+15, y+110, 14, rl.LightGray)

	// Líquidos
	if tile.WaterLevel > 0 {
		rl.DrawText(fmt.Sprintf("Água: %d/7", tile.WaterLevel), x+15, y+115, 16, rl.SkyBlue)
	}
	if tile.MagmaLevel > 0 {
		rl.DrawText(fmt.Sprintf("Magma: %d/7", tile.MagmaLevel), x+15, y+135, 16, rl.Red)
	}

	// Misc
	if tile.Hidden {
		rl.DrawText("[ESCONDIDO]", x+15, y+155, 14, rl.DarkGray)
	}
}

// drawPauseMenu desenha o menu de escape centralizado.
func (a *App) drawPauseMenu() {
	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// 1. Fundo escurecido (Dimmer)
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(0, 0, 0, 150))

	// 2. Painel Central
	panelWidth := int32(400)
	panelHeight := int32(300)
	panelX := (screenWidth - panelWidth) / 2
	panelY := (screenHeight - panelHeight) / 2

	rl.DrawRectangle(panelX, panelY, panelWidth, panelHeight, rl.NewColor(30, 30, 35, 255))
	rl.DrawRectangleLines(panelX, panelY, panelWidth, panelHeight, rl.White)

	// Título do Menu
	menuTitle := "MENU DE PAUSA"
	titleWidth := rl.MeasureText(menuTitle, 24)
	rl.DrawText(menuTitle, panelX+(panelWidth-titleWidth)/2, panelY+30, 24, rl.Gold)

	// 3. Botões
	buttonX := panelX + 50
	buttonWidth := panelWidth - 100
	buttonHeight := int32(40)

	// Botão: RETOMAR
	if a.drawButton(buttonX, panelY+90, buttonWidth, buttonHeight, "RETOMAR (ESC)", rl.Green) {
		a.State = StateViewing
	}

	// Botão: CONFIGURAÇÕES (Placeholder/Info)
	if a.drawButton(buttonX, panelY+145, buttonWidth, buttonHeight, "OPÇÕES (F3/F4/F7)", rl.Gray) {
		// Por enquanto exibe apenas info, mas poderia abrir submenu
	}

	// Botão: SAIR
	if a.drawButton(buttonX, panelY+200, buttonWidth, buttonHeight, "SAIR DO JOGO", rl.Red) {
		// Para fechar via código no Raylib/Go, precisamos sinalizar o loop principal
		// mas aqui podemos apenas chamar o cleanup e sair
		a.shutdown()
		log.Println("[App] Encerrando aplicação pelo menu.")
		runtime.Goexit() // Uma forma de "parar" forçado se necessário, mas WindowShouldClose é melhor.
		// Vamos usar uma flag ou apenas forçar o fechamento da janela
		rl.CloseWindow()
	}
}

// drawButton desenha um botão genérico com hover e retorna true se clicado.
func (a *App) drawButton(x, y, w, h int32, text string, color rl.Color) bool {
	mousePos := rl.GetMousePosition()
	isHover := mousePos.X >= float32(x) && mousePos.X <= float32(x+w) &&
		mousePos.Y >= float32(y) && mousePos.Y <= float32(y+h)

	drawColor := color
	if isHover {
		drawColor.R += 30
		drawColor.G += 30
		drawColor.B += 30
		rl.SetMouseCursor(rl.MouseCursorPointingHand)
	} else {
		rl.SetMouseCursor(rl.MouseCursorDefault)
	}

	rl.DrawRectangle(x, y, w, h, rl.NewColor(50, 50, 50, 255))
	rl.DrawRectangleLines(x, y, w, h, drawColor)

	textWidth := rl.MeasureText(text, 18)
	rl.DrawText(text, x+(w-textWidth)/2, y+(h-18)/2, 18, rl.White)

	return isHover && rl.IsMouseButtonPressed(rl.MouseLeftButton)
}

func (a *App) drawLoadingScreen() {
	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// Fundo
	rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.NewColor(20, 20, 25, 255))

	// Título
	title := "FORTRESSVISION"
	titleWidth := rl.MeasureText(title, 40)
	rl.DrawText(title, (screenWidth-titleWidth)/2, screenHeight/2-60, 40, rl.Gold)

	// Desenha barra de progresso
	barWidth := int32(400)
	barHeight := int32(30)
	barX := (screenWidth - barWidth) / 2
	barY := screenHeight/2 + 20

	rl.DrawRectangle(barX, barY, barWidth, barHeight, rl.DarkGray)
	rl.DrawRectangle(barX, barY, int32(float32(barWidth)*a.LoadingProgress), barHeight, rl.Orange)
	rl.DrawRectangleLines(barX, barY, barWidth, barHeight, rl.White)

	// Status
	statusWidth := rl.MeasureText(a.LoadingStatus, 18)
	rl.DrawText(a.LoadingStatus, (screenWidth-statusWidth)/2, barY+45, 18, rl.LightGray)

	// Rodapé
	tip := "Pressione ESPAÇO para entrar imediatamente (pode haver vácuo)."
	if a.FullScanActive {
		tip = "Varredura total em curso... Aguarde a conclusão para evitar falhas no terreno."
	}
	tipWidth := rl.MeasureText(tip, 16)
	rl.DrawText(tip, (screenWidth-tipWidth)/2, screenHeight-50, 16, rl.Gray)

	// Raylib bug fix: Durante o loading, processar eventos mantém a janela responsiva
	if rl.IsWindowResized() {
		// Atualiza dimensões se necessário
	}
}
