package camera

import (
	"math"

	"kaijuengine.com/engine"
	"kaijuengine.com/matrix"
	"kaijuengine.com/platform/hid"
)

// Controller gerencia a camera orbital do FortressVision (Kaiju Engine).
// Coordenadas esfericas: Distance, Yaw (horizontal), Pitch (vertical).
type Controller struct {
	host *engine.Host

	// Alvo de visualizacao (interpolado com lerp)
	TargetLookAt  [3]float32
	CurrentLookAt [3]float32

	// Parametros esfericos
	TargetDistance  float32
	CurrentDistance float32
	Yaw            float32 // Rotacao horizontal em radianos
	Pitch          float32 // Rotacao vertical em radianos

	// Limites
	MinPitch    float32
	MaxPitch    float32
	MinDistance float32
	MaxDistance float32
	MoveSpeed   float32
	RotateSpeed float32
	ZoomSpeed   float32

	// Interpolacao
	LerpSpeed float32

	// Estado do mouse para drag
	lastMouseX   float32
	lastMouseY   float32
	isDragging   bool
	isMiddleDrag bool
	lastHoldQ    bool
	lastHoldE    bool
	repeatTimerQ float32
	repeatTimerE float32
}

// New cria um controlador de camera orbital para Kaiju Engine.
func New(host *engine.Host) *Controller {
	c := &Controller{
		host:            host,
		TargetLookAt:    [3]float32{16, 0, 16},
		CurrentLookAt:   [3]float32{16, 0, 16},
		TargetDistance:  40.0,
		CurrentDistance: 40.0,
		Yaw:            -math.Pi / 4.0,
		Pitch:          math.Pi / 6.0,
		MinPitch:       0.1,
		MaxPitch:       math.Pi/2.0 - 0.05,
		MinDistance:    5.0,
		MaxDistance:    500.0,
		MoveSpeed:     30.0,
		RotateSpeed:   0.005,
		ZoomSpeed:     5.0,
		LerpSpeed:     8.0,
	}

	c.apply()
	return c
}

// HandleInput processa WASD, Scroll e Mouse Drag.
func (c *Controller) HandleInput(dt float32, kbd *hid.Keyboard, mouse *hid.Mouse) {
	// Vetores da direcao da camera:
	forwardX := float32(math.Cos(float64(c.Yaw)))
	forwardZ := float32(math.Sin(float64(c.Yaw)))
	
	rightX := forwardZ
	rightZ := -forwardX

	// Velocidade baseada na ALTURA atual (qto mais alto, mais rapido anda)
	// Como em Timberborn e RTS modernos
	dynamicSpeed := (c.CurrentDistance * 1.2) * dt
	if dynamicSpeed < 10.0*dt {
		dynamicSpeed = 10.0 * dt // velocidade minima
	}

	if kbd.HasShift() {
		dynamicSpeed *= 2.5
	}

	// WASD - Movimento paralelo ao chao
	if kbd.KeyHeld(hid.KeyboardKeyW) {
		c.TargetLookAt[0] -= forwardX * dynamicSpeed
		c.TargetLookAt[2] -= forwardZ * dynamicSpeed
	}
	if kbd.KeyHeld(hid.KeyboardKeyS) {
		c.TargetLookAt[0] += forwardX * dynamicSpeed
		c.TargetLookAt[2] += forwardZ * dynamicSpeed
	}
	if kbd.KeyHeld(hid.KeyboardKeyA) {
		c.TargetLookAt[0] -= rightX * dynamicSpeed
		c.TargetLookAt[2] -= rightZ * dynamicSpeed
	}
	if kbd.KeyHeld(hid.KeyboardKeyD) {
		c.TargetLookAt[0] += rightX * dynamicSpeed
		c.TargetLookAt[2] += rightZ * dynamicSpeed
	}

	// Q/E - Subir/Descer o foco no eixo Y vertical (Pulso Único + Repetição)
	const repeatDelay = 0.4  // Atraso inicial antes de começar a repetir (segundos)
	const repeatRate = 0.05  // Intervalo entre repetições ao segurar (segundos)

	// Lógica para E (Subir)
	isHeldE := kbd.KeyHeld(hid.KeyboardKeyE)
	if isHeldE {
		if !c.lastHoldE {
			// Clique inicial: sobe 1 nível imediatamente
			c.TargetLookAt[1] += 1.0
			c.repeatTimerE = 0
		} else {
			// Segurando: incrementa timer
			c.repeatTimerE += dt
			if c.repeatTimerE > repeatDelay {
				// Começa a repetir após o delay inicial
				if c.repeatTimerE > repeatDelay+repeatRate {
					c.TargetLookAt[1] += 1.0
					c.repeatTimerE = repeatDelay // Reseta para o próximo pulso de repetição
				}
			}
		}
	}
	c.lastHoldE = isHeldE

	// Lógica para Q (Descer)
	isHeldQ := kbd.KeyHeld(hid.KeyboardKeyQ)
	if isHeldQ {
		if !c.lastHoldQ {
			// Clique inicial: desce 1 nível imediatamente
			c.TargetLookAt[1] -= 1.0
			c.repeatTimerQ = 0
		} else {
			// Segurando: incrementa timer
			c.repeatTimerQ += dt
			if c.repeatTimerQ > repeatDelay {
				if c.repeatTimerQ > repeatDelay+repeatRate {
					c.TargetLookAt[1] -= 1.0
					c.repeatTimerQ = repeatDelay
				}
			}
		}
	}
	c.lastHoldQ = isHeldQ

	// Zoom - Scroll Wheel EXPONENCIAL
	// Zoom de perto é suave, zoom de longe é veloz
	if mouse.Scrolled() {
		zoomFactor := float32(1.0)
		if mouse.ScrollY > 0 {
			zoomFactor = 0.85 // aproxima 15% por scroll
		} else if mouse.ScrollY < 0 {
			zoomFactor = 1.15 // afasta 15% por scroll
		}
		
		c.TargetDistance *= zoomFactor

		if c.TargetDistance < c.MinDistance {
			c.TargetDistance = c.MinDistance
		}
		if c.TargetDistance > c.MaxDistance {
			c.TargetDistance = c.MaxDistance
		}
	}

	// Rotacao (Botao Direito) - OrbitGrab
	if mouse.Pressed(hid.MouseButtonRight) {
		c.isDragging = true
		c.lastMouseX = mouse.SX
		c.lastMouseY = mouse.SY
	}
	if c.isDragging && (mouse.Held(hid.MouseButtonRight) || mouse.Pressed(hid.MouseButtonRight)) {
		if mouse.Moved() {
			dx := mouse.SX - c.lastMouseX
			dy := mouse.SY - c.lastMouseY
			
			// Invertido a pedido do usuario:
			c.Yaw += dx * c.RotateSpeed
			c.Pitch += dy * c.RotateSpeed

			if c.Pitch < c.MinPitch {
				c.Pitch = c.MinPitch
			}
			if c.Pitch > c.MaxPitch {
				c.Pitch = c.MaxPitch
			}

			c.lastMouseX = mouse.SX
			c.lastMouseY = mouse.SY
		}
	}
	if mouse.Released(hid.MouseButtonRight) {
		c.isDragging = false
	}

	// Pan (Botao do Meio)
	if mouse.Pressed(hid.MouseButtonMiddle) {
		c.isMiddleDrag = true
		c.lastMouseX = mouse.SX
		c.lastMouseY = mouse.SY
	}
	if c.isMiddleDrag && (mouse.Held(hid.MouseButtonMiddle) || mouse.Pressed(hid.MouseButtonMiddle)) {
		if mouse.Moved() {
			dx := mouse.SX - c.lastMouseX
			dy := mouse.SY - c.lastMouseY
			
			// Velocidade de arrasto adaptativa à altura da câmera
			panSpeed := c.CurrentDistance * 0.003

			// Move o target na direcao do arraste (invertido em relacao ao grab original)
			c.TargetLookAt[0] += rightX * dx * panSpeed
			c.TargetLookAt[2] += rightZ * dx * panSpeed

			c.TargetLookAt[0] += forwardX * dy * panSpeed
			c.TargetLookAt[2] += forwardZ * dy * panSpeed

			c.lastMouseX = mouse.SX
			c.lastMouseY = mouse.SY
		}
	}
	if mouse.Released(hid.MouseButtonMiddle) {
		c.isMiddleDrag = false
	}
}

// Update interpola e aplica a camera. Deve ser chamado a cada frame apos HandleInput.
func (c *Controller) Update(dt float32) {
	// Lerp frame-rate independent exponencial
	// O fator determina o quão rápido a câmera "alcanca" o alvo
	t := float32(1.0 - math.Exp(float64(-c.LerpSpeed * dt)))

	// Interpolar posicao do alvo
	c.CurrentLookAt[0] += (c.TargetLookAt[0] - c.CurrentLookAt[0]) * t
	c.CurrentLookAt[1] += (c.TargetLookAt[1] - c.CurrentLookAt[1]) * t
	c.CurrentLookAt[2] += (c.TargetLookAt[2] - c.CurrentLookAt[2]) * t

	// Interpolar distancia
	c.CurrentDistance += (c.TargetDistance - c.CurrentDistance) * t

	// Aplicar transformacao na camera
	c.apply()
}

// apply converte coordenadas esfericas e aplica na camera primaria da Kaiju.
func (c *Controller) apply() {
	posX := c.CurrentLookAt[0] + c.CurrentDistance*float32(math.Cos(float64(c.Pitch)))*float32(math.Cos(float64(c.Yaw)))
	posY := c.CurrentLookAt[1] + c.CurrentDistance*float32(math.Sin(float64(c.Pitch)))
	posZ := c.CurrentLookAt[2] + c.CurrentDistance*float32(math.Cos(float64(c.Pitch)))*float32(math.Sin(float64(c.Yaw)))

	cam := c.host.PrimaryCamera()
	cam.SetPosition(matrix.NewVec3(matrix.Float(posX), matrix.Float(posY), matrix.Float(posZ)))

	lookAt := matrix.NewVec3(matrix.Float(c.CurrentLookAt[0]), matrix.Float(c.CurrentLookAt[1]), matrix.Float(c.CurrentLookAt[2]))
	cam.SetLookAt(lookAt)
}
