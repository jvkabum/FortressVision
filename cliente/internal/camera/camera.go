package camera

import (
	"math"

	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

// Mode define o tipo de projeção estritamente.
type Mode int

const (
	ModePerspective Mode = iota
	ModeOrthographic
)

// CameraController gerencia a lógica de movimentação e projeção da câmera.
// Segue o estilo do Armok Vision: movimento suave e zoom que afeta a velocidade.
type CameraController struct {
	// Estado interno do Raylib
	RLCamera rl.Camera3D

	// Configurações
	Mode           Mode
	TargetDistance float32 // Distância desejada do alvo (Zoom)
	MinZoom        float32
	MaxZoom        float32
	MoveSpeed      float32
	RotateSpeed    float32
	ZoomSpeed      float32
	SmoothFactor   float32 // 0.0 a 1.0 (quanto menor, mais suave/lento)

	// Estado Alvo (para interpolação suave)
	TargetPos    rl.Vector3 // Onde a câmera quer chegar (posição real no mundo)
	TargetLookAt rl.Vector3 // Para onde a câmera quer olhar (ponto central)
	TargetZoom   float32    // Zoom desejado
	TargetAngleY float32    // Rotação horizontal atual (radianos)
	TargetAngleX float32    // Rotação vertical atual (radianos)

	// Estado Atual (interpolado)
	CurrentLookAt rl.Vector3
	CurrentZoom   float32
}

// New cria um novo controlador de câmera.
func New() *CameraController {
	c := &CameraController{
		Mode:         ModePerspective,
		MinZoom:      5.0,
		MaxZoom:      200.0,
		MoveSpeed:    50.0,
		RotateSpeed:  2.0,
		ZoomSpeed:    10.0,
		SmoothFactor: 0.1, // Ajuste fino para sensação de peso

		// Valores iniciais
		TargetLookAt: rl.Vector3{X: 0, Y: 0, Z: 0},
		TargetZoom:   50.0,
		TargetAngleY: 45.0 * rl.Deg2rad,  // 45 graus (padrão isométrico)
		TargetAngleX: -30.0 * rl.Deg2rad, // -30 graus (olhando de cima)
	}

	// Inicializa os valores atuais com os alvos para não "saltar" no início
	c.CurrentLookAt = c.TargetLookAt
	c.CurrentZoom = c.TargetZoom

	c.RLCamera = rl.Camera3D{
		Up:         rl.Vector3{X: 0, Y: 1, Z: 0},
		Fovy:       45.0,
		Projection: rl.CameraPerspective,
	}

	c.UpdateWait(1.0) // Força atualização imediata da posição
	return c
}

// SetPosition define a posição do alvo da câmera imediatamente (sem suavização).
func (c *CameraController) SetTarget(pos rl.Vector3) {
	c.TargetLookAt = pos
	c.CurrentLookAt = pos
	c.UpdateWait(1.0)
}

// Update calcula a nova posição da câmera com base no tempo (dt).
// Deve ser chamado a cada frame.
func (c *CameraController) Update(dt float32) {
	// Interpolação suave (Lerp)
	// Usamos uma fórmula de amortecimento independente de frame rate:
	// val = Lerp(val, target, 1 - exp(-speed * dt))
	// Mas para simplicidade e controle linear:
	factor := c.SmoothFactor * 60.0 * dt // Normaliza para 60 FPS
	if factor > 1.0 {
		factor = 1.0
	}

	// Conversão rl.Vector3 -> mgl32.Vec3 para interpolação VectorLerp segura
	curVec := mgl32.Vec3{c.CurrentLookAt.X, c.CurrentLookAt.Y, c.CurrentLookAt.Z}
	tgtVec := mgl32.Vec3{c.TargetLookAt.X, c.TargetLookAt.Y, c.TargetLookAt.Z}

	lerpedVec := curVec.Add(tgtVec.Sub(curVec).Mul(factor)) // Lerp manual MGL32

	c.CurrentLookAt = rl.Vector3{X: lerpedVec.X(), Y: lerpedVec.Y(), Z: lerpedVec.Z()}
	c.CurrentZoom = util.Lerp(c.CurrentZoom, c.TargetZoom, factor)

	c.UpdateWait(dt)
}

// UpdateWait recálcula a posição da câmera baseada nos ângulos e zoom atuais.
func (c *CameraController) UpdateWait(dt float32) {
	// Converte coordenadas esféricas para cartesianas
	// X = r * sin(theta) * cos(phi)
	// Y = r * sin(phi)
	// Z = r * cos(theta) * cos(phi)

	dist := c.CurrentZoom

	// No modo ortográfico, a distância física da câmera não altera o tamanho do objeto,
	// mas precisamos afastar a câmera para não cortar a geometria (near plane).
	// O "Zoom" no ortográfico é controlado pelo Fovy (escala).
	if c.Mode == ModeOrthographic {
		// Ajuste mágico para manter a escala visual parecida ao trocar de modo
		c.RLCamera.Fovy = c.CurrentZoom * 0.5
		c.RLCamera.Projection = rl.CameraOrthographic
		dist = 200.0 // Mantém a câmera longe para evitar clipping
	} else {
		c.RLCamera.Fovy = 45.0
		c.RLCamera.Projection = rl.CameraPerspective
	}

	// Calcula offset baseado nos ângulos (usando matemática mais robusta se necessário futuramente, mas mantendo a conversão esférica simples)
	// AngleY = Rotação em torno do eixo Y (Azimute)
	// AngleX = Elevação (Latitude)
	cosX := float32(math.Cos(float64(c.TargetAngleX)))
	sinX := float32(math.Sin(float64(c.TargetAngleX)))
	cosY := float32(math.Cos(float64(c.TargetAngleY)))
	sinY := float32(math.Sin(float64(c.TargetAngleY)))

	offsetX := dist * cosX * sinY
	offsetY := dist * -sinX // Y é UP no Raylib, sinX negativo pois olhamos de cima para baixo
	offsetZ := dist * cosX * cosY

	c.RLCamera.Position = rl.Vector3{
		X: c.CurrentLookAt.X + offsetX,
		Y: c.CurrentLookAt.Y + offsetY,
		Z: c.CurrentLookAt.Z + offsetZ,
	}

	c.RLCamera.Target = c.CurrentLookAt
}

// SetMode alterna entre Perspectiva e Ortográfica.
func (c *CameraController) SetMode(mode Mode) {
	c.Mode = mode
	// Ao trocar, recalculamos imediatamente para evitar frames estranhos
	c.UpdateWait(0)
}

// InputHandles processa entrada do usuário. Retorna true se houve input de movimento.
func (c *CameraController) HandleInput(dt float32) bool {
	moved := false
	// Zoom com Scroll
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		moved = true
		// Zoom logarítmico ou acelerado
		c.TargetZoom -= wheel * c.ZoomSpeed
		if c.TargetZoom < c.MinZoom {
			c.TargetZoom = c.MinZoom
		}
		if c.TargetZoom > c.MaxZoom {
			c.TargetZoom = c.MaxZoom
		}
	}

	// Rotação com botão esquerdo (Orbit)
	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		delta := rl.GetMouseDelta()
		if delta.X != 0 || delta.Y != 0 {
			moved = true
		}
		c.TargetAngleY -= delta.X * c.RotateSpeed * 0.005
		c.TargetAngleX -= delta.Y * c.RotateSpeed * 0.005

		// Clamp na elevação para não virar a câmera de ponta cabeça
		// Limite entre -89 graus (quase topo) e -5 graus (quase horizonte)
		maxElev := -5.0 * rl.Deg2rad
		minElev := -89.0 * rl.Deg2rad
		if c.TargetAngleX > float32(maxElev) {
			c.TargetAngleX = float32(maxElev)
		}
		if c.TargetAngleX < float32(minElev) {
			c.TargetAngleX = float32(minElev)
		}
	}

	// Movimento WASD (Relativo à câmera) transformado para usar mgl32
	camPos := mgl32.Vec3{c.RLCamera.Position.X, c.RLCamera.Position.Y, c.RLCamera.Position.Z}
	targetPos := mgl32.Vec3{c.TargetLookAt.X, c.TargetLookAt.Y, c.TargetLookAt.Z}

	// Precisamos calcular os vetores Forward e Right projetados no plano XZ (chão)
	forward := targetPos.Sub(camPos)
	forward[1] = 0 // forward.Y = 0
	forward = forward.Normalize()

	upVec := mgl32.Vec3{0, 1, 0}
	right := forward.Cross(upVec).Normalize()

	// Velocidade baseada no zoom (como no Armok Vision)
	// Quanto mais alto, mais rápido.
	currentSpeed := c.MoveSpeed * (c.CurrentZoom / 50.0) * dt

	moveMove := mgl32.Vec3{0, 0, 0}

	if rl.IsKeyDown(rl.KeyW) {
		moveMove = moveMove.Add(forward)
	}
	if rl.IsKeyDown(rl.KeyS) {
		moveMove = moveMove.Sub(forward)
	}
	if rl.IsKeyDown(rl.KeyD) {
		moveMove = moveMove.Add(right)
	}
	if rl.IsKeyDown(rl.KeyA) {
		moveMove = moveMove.Sub(right)
	}

	if moveMove.Len() > 0 {
		moveMove = moveMove.Normalize().Mul(currentSpeed)
		targetPos = targetPos.Add(moveMove)

		c.TargetLookAt = rl.Vector3{
			X: targetPos.X(),
			Y: targetPos.Y(),
			Z: targetPos.Z(),
		}
		moved = true
	}

	return moved
}
