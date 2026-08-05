package hud

import (
	"fmt"
	"math"

	"kaijuengine.com/engine"
	"kaijuengine.com/engine/assets"
	"kaijuengine.com/engine/ui"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
)

// ============================================================================
// Cores pré-alocadas
// ============================================================================
var (
	colorDarkBg    = matrix.NewColor(0.10, 0.10, 0.12, 1.00)
	colorInspectBg = matrix.NewColor(0.12, 0.12, 0.14, 1.00)
	colorBorderCy  = matrix.NewColor(0.25, 0.55, 0.65, 0.50)
	colorTitle     = matrix.NewColor(0.95, 0.95, 0.97, 1.00)
	colorMenta     = matrix.NewColor(0.30, 0.92, 0.50, 1.00)
	colorCyan      = matrix.NewColor(0.55, 0.82, 0.95, 1.00)
	colorAmbar     = matrix.NewColor(0.95, 0.75, 0.30, 1.00)
	colorGray      = matrix.NewColor(0.58, 0.60, 0.68, 1.00)
	colorHint      = matrix.NewColor(0.42, 0.44, 0.50, 1.00)
	colorInspHead  = matrix.NewColor(0.35, 0.75, 0.85, 1.00)
	colorInspect   = matrix.NewColor(0.50, 0.90, 1.00, 1.00)
	colorMagma     = matrix.NewColor(1.00, 0.45, 0.20, 1.00) // Laranja magma
	colorAgua      = matrix.NewColor(0.30, 0.65, 1.00, 1.00) // Azul água
	colorLabel     = matrix.NewColor(0.55, 0.60, 0.70, 0.90) // Cinza azulado sutil (labels)
	colorValue     = matrix.NewColor(0.45, 0.85, 1.00, 1.00) // Azul elétrico (valores)
)

const (
	lineH  float32 = 26
	padL   float32 = 14
	padT   float32 = 14
	margin float32 = 16
)

// ============================================================================
// HUD — 5 painéis distribuídos nos 4 cantos (responsivo)
// ============================================================================
type HUD struct {
	man  ui.Manager
	host *engine.Host

	// Painéis que precisam reposicionar no resize
	pnlWorld    *ui.Panel
	pnlInspect  *ui.Panel
	pnlLocation *ui.Panel
	pnlControls *ui.Panel

	// Dimensões dos painéis (para recalcular posição)
	worldW, worldH float32
	inspW, inspH   float32
	locW, locH     float32
	ctrlW, ctrlH   float32

	// Labels dinâmicas
	lblFPS       *ui.Label
	lblWorldName *ui.Label
	lblWeather   *ui.Label
	lblStats     *ui.Label
	lblPop       *ui.Label
	lblInspHead  *ui.Label
	lblInspItem  *ui.Label
	lblInspMat   *ui.Label
	lblInspType  *ui.Label
	lblInspAgua  *ui.Label
	lblInspMagma *ui.Label
	lblLocHead   *ui.Label
	lblLocDF     *ui.Label
	lblLocElev   *ui.Label

	timer      float64
	frameCount int
	lastW      int
	lastH      int
	zOffset    int32
}

func New(host *engine.Host) *HUD {
	h := &HUD{host: host}
	h.man.Init(host)

	tex, _ := host.TextureCache().Texture(
		assets.TextureSquare, rendering.TextureFilterLinear,
	)

	// ═════════════════════════════════════════════════════════════════
	// 1) FPS — topo-esquerda (texto flutuante, sem caixa)
	// ═════════════════════════════════════════════════════════════════
	clearBg := matrix.NewColor(0, 0, 0, 0)
	fpsW := float32(160)
	fpsH := padT + 28
	fpsPanel := h.makePanel(tex, fpsW, fpsH, 8, 8, clearBg)
	fpsPanel.Base().ShaderData().BorderSize = matrix.Vec4{0, 0, 0, 0} // sem borda

	h.lblFPS = h.addRow(fpsPanel, "FPS: --", 18, colorMenta, 0, 0)
	// FPS sem caixa: forçar bgColor transparente (sem retângulo preto)
	h.lblFPS.SetBGColor(matrix.NewColor(0, 0, 0, 0))

	// ═════════════════════════════════════════════════════════════════
	// 2) Mundo — topo-direita
	// ═════════════════════════════════════════════════════════════════
	var row float32
	h.worldW = 140
	h.worldH = padT + 28 + lineH*3 + padT
	h.pnlWorld = h.makePanel(tex, h.worldW, h.worldH, 0, margin, colorDarkBg)

	row = padT
	h.lblWorldName = h.addRowColor(h.pnlWorld, "Fortress Vision", 16, colorTitle, padL, row)
	row += 28
	h.lblWeather = h.addRowStat(h.pnlWorld, "Clima:", "--", padL, row)
	row += lineH
	h.lblStats = h.addRowStat(h.pnlWorld, "Estação:", "--", padL, row)
	row += lineH
	h.lblPop = h.addRowStat(h.pnlWorld, "POP:", "--", padL, row)
	h.lblPop.SetColor(colorAmbar)

	// ═════════════════════════════════════════════════════════════════
	// 3) Inspeção — centro-direita
	// ═════════════════════════════════════════════════════════════════
	h.inspW = 230
	h.inspH = padT + 22 + lineH*5 + padT
	h.pnlInspect = h.makePanel(tex, h.inspW, h.inspH, 0, 0, colorInspectBg)

	row = padT
	h.lblInspHead = h.addRowColor(h.pnlInspect, "INSPEÇÃO", 10, colorInspHead, padL, row)
	row += 22
	h.lblInspItem = h.addRowStat(h.pnlInspect, "Item:", "--", padL, row)
	row += lineH
	h.lblInspMat = h.addRowStat(h.pnlInspect, "Material:", "--", padL, row)
	row += lineH
	h.lblInspType = h.addRowStat(h.pnlInspect, "Tipo:", "--", padL, row)
	row += lineH
	h.lblInspAgua = h.addRowStat(h.pnlInspect, "Água:", "0/7", padL, row)
	h.lblInspAgua.SetColor(colorAgua)
	row += lineH
	h.lblInspMagma = h.addRowStat(h.pnlInspect, "Magma:", "0/7", padL, row)
	h.lblInspMagma.SetColor(colorMagma)

	// ═════════════════════════════════════════════════════════════════
	// 4) Localização — inferior-esquerda
	// ═════════════════════════════════════════════════════════════════
	h.locW = 115
	h.locH = padT + 22 + lineH*2 + padT
	h.pnlLocation = h.makePanel(tex, h.locW, h.locH, margin, 0, colorDarkBg)

	row = padT
	h.lblLocHead = h.addRow(h.pnlLocation, "LOCALIZAÇÃO", 10, colorInspHead, padL, row)
	row += 22
	h.lblLocDF = h.addRow(h.pnlLocation, "DF: --, --, --", 14, colorCyan, padL, row)
	row += lineH
	h.lblLocElev = h.addRow(h.pnlLocation, "Elevação: --", 14, colorCyan, padL, row)

	// ═════════════════════════════════════════════════════════════════
	// 5) Controles — inferior-direita
	// ═════════════════════════════════════════════════════════════════
	h.ctrlW = 100
	h.ctrlH = padT + 22 + lineH*4 + padT
	h.pnlControls = h.makePanel(tex, h.ctrlW, h.ctrlH, 0, 0, colorDarkBg)

	row = padT
	h.addRowColor(h.pnlControls, "CONTROLES", 10, colorInspHead, padL, row)
	row += 22
	h.addRowKey(h.pnlControls, "WASD", "Mover", padL, row)
	row += lineH
	h.addRowKey(h.pnlControls, "Q/E", "Nível Z", padL, row)
	row += lineH
	h.addRowKey(h.pnlControls, "SCL", "Zoom", padL, row)
	row += lineH
	h.addRowKey(h.pnlControls, "F3", "HUD", padL, row)

	// Posicionar painéis responsivos na primeira vez
	h.repositionPanels()

	return h
}

// repositionPanels recalcula a posição dos painéis da direita e de baixo
// com base nas dimensões ATUAIS da janela.
func (h *HUD) repositionPanels() {
	scrW := float32(h.host.Window.Width())
	scrH := float32(h.host.Window.Height())

	// Mundo — topo-direita
	h.pnlWorld.Base().Layout().SetOffset(scrW-margin-h.worldW, margin)

	// Inspeção — centro-direita (40% da altura)
	h.pnlInspect.Base().Layout().SetOffset(scrW-margin-h.inspW, scrH*0.38)

	// Localização — inferior-esquerda
	h.pnlLocation.Base().Layout().SetOffset(margin, scrH-margin-h.locH)

	// Controles — inferior-direita
	h.pnlControls.Base().Layout().SetOffset(scrW-margin-h.ctrlW, scrH-margin-h.ctrlH)

	h.lastW = h.host.Window.Width()
	h.lastH = h.host.Window.Height()
}

// ============================================================================
// Helpers de construção
// ============================================================================

func (h *HUD) makePanel(tex *rendering.Texture, w, hght, x, y float32, bg matrix.Color) *ui.Panel {
	p := h.man.Add().ToPanel()
	p.Init(tex, ui.ElementTypePanel)
	p.SetColor(bg)
	p.DontFitContent()
	p.Base().Layout().SetPositioning(ui.PositioningAbsolute)
	p.Base().Layout().Scale(w, hght)
	p.Base().Layout().SetOffset(x, y)
	p.Base().ShaderData().BorderRadius = matrix.Vec4{10, 10, 10, 10}
	p.Base().ShaderData().BorderSize = matrix.Vec4{1, 1, 1, 1}
	p.Base().ShaderData().BorderColor[0] = colorBorderCy
	return p
}

func (h *HUD) addRow(parent *ui.Panel, text string, size float32, color matrix.Color, x, y float32) *ui.Label {
	return h.addRowColor(parent, text, size, color, x, y)
}

// addRowColor cria um label simples com cor específica.
func (h *HUD) addRowColor(parent *ui.Panel, text string, size float32, color matrix.Color, x, y float32) *ui.Label {
	lbl := h.man.Add().ToLabel()
	lbl.Init(text)
	lbl.SetFontSize(size)
	lbl.SetColor(color)
	parentBg := parent.Color()
	lbl.SetBGColor(matrix.NewColor(parentBg.R(), parentBg.G(), parentBg.B(), 1.0))
	lbl.Base().Layout().SetPositioning(ui.PositioningAbsolute)
	lbl.Base().Layout().Scale(1000, 22) // largura grande para não cortar
	lbl.Base().Layout().SetOffset(x, y)
	parent.AddChild(lbl.Base())
	return lbl
}

// addRowStat cria uma linha organizada com "Etiqueta: Valor" em cores diferentes.
func (h *HUD) addRowStat(parent *ui.Panel, stat, value string, x, y float32) *ui.Label {
	// Label da estatística (dimmed)
	h.addRowColor(parent, stat, 11, colorLabel, x, y)
	// Valor da estatística (bright, offset lateral)
	valX := x + 55 // offset fixo para o valor
	if len(stat) > 8 {
		valX += 10
	}
	return h.addRowColor(parent, value, 12, colorValue, valX, y)
}

// addRowKey cria uma linha para controles com a tecla em destaque.
func (h *HUD) addRowKey(parent *ui.Panel, key, desc string, x, y float32) *ui.Label {
	h.addRowColor(parent, key, 10, colorTitle, x, y)
	return h.addRowColor(parent, desc, 11, colorHint, x+40, y)
}

func (h *HUD) set(lbl *ui.Label, text string) {
	lbl.SetText(text)
	lbl.Base().SetDirty(ui.DirtyTypeLayout)
}

// ============================================================================
// Update — chamado a cada frame
// ============================================================================
func (h *HUD) Update(dt float64, lookAt matrix.Vec3) {
	// Reposicionar se a janela mudou de tamanho
	if h.host.Window.Width() != h.lastW || h.host.Window.Height() != h.lastH {
		h.repositionPanels()
	}

	h.frameCount++
	h.timer += dt
	if h.timer >= 0.5 {
		h.UpdateFPS(int(float64(h.frameCount) / h.timer))
		h.UpdateLocation(
			float64(lookAt[0]),
			float64(lookAt[1]),
			float64(lookAt[2]),
		)
		h.timer = 0
		h.frameCount = 0
	}
}

// ============================================================================
// Métodos públicos
// ============================================================================

func (h *HUD) UpdateFPS(fps int) {
	h.set(h.lblFPS, fmt.Sprintf("FPS: %d", fps))
}

func (h *HUD) UpdateLocation(x, y, z float64) {
	dfY := -z
	h.set(h.lblLocDF, fmt.Sprintf("DF: %.0f, %.0f, %.0f", x, dfY, y))
	elevation := int(math.Floor(y)) - int(h.zOffset)
	h.set(h.lblLocElev, fmt.Sprintf("Elevação: %d", elevation))
}

func (h *HUD) SetZOffset(offset int32) {
	h.zOffset = offset
}

func (h *HUD) UpdatePop(pop int) {
	h.set(h.lblPop, fmt.Sprintf("%d", pop))
}

func (h *HUD) UpdateWorld(name string) {
	h.set(h.lblWorldName, name)
}

func (h *HUD) UpdateWeather(icon, text string) {
	h.set(h.lblWeather, fmt.Sprintf("%s %s", icon, text))
}

func (h *HUD) UpdateSeason(season string) {
	h.set(h.lblStats, season)
}

func (h *HUD) UpdateInspect(mat, typ string, agua, magma int) {
	h.set(h.lblInspMat, mat)
	h.set(h.lblInspType, typ)
	h.set(h.lblInspAgua, fmt.Sprintf("%d/7", agua))
	h.set(h.lblInspMagma, fmt.Sprintf("%d/7", magma))
}

// UpdateInspectItem atualiza o item selecionado pelo clique esquerdo.
func (h *HUD) UpdateInspectItem(name string, stackSize int32) {
	if name == "" {
		name = "--"
	} else if stackSize > 1 {
		name = fmt.Sprintf("%s x%d", name, stackSize)
	}
	if len([]rune(name)) > 27 {
		name = string([]rune(name)[:27]) + "..."
	}
	h.set(h.lblInspItem, name)
}
