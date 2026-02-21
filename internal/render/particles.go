package render

import (
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type Particle struct {
	Position rl.Vector3
	Velocity rl.Vector3
	Alpha    float32
	Active   bool
}

type WeatherType int

const (
	WeatherNone WeatherType = iota
	WeatherRain
	WeatherSnow
)

type ParticleSystem struct {
	Particles    []Particle
	MaxParticles int
	Type         WeatherType
}

func NewParticleSystem(max int) *ParticleSystem {
	ps := &ParticleSystem{
		Particles:    make([]Particle, max),
		MaxParticles: max,
		Type:         WeatherSnow, // Padrão
	}
	for i := 0; i < max; i++ {
		ps.resetParticle(i)
	}
	return ps
}

func (ps *ParticleSystem) resetParticle(i int) {
	ps.Particles[i].Position = rl.Vector3{
		X: rand.Float32()*200 - 100,
		Y: rand.Float32()*50 + 20,
		Z: rand.Float32()*200 - 100,
	}
	if ps.Type == WeatherRain {
		ps.Particles[i].Velocity = rl.Vector3{X: 0, Y: -20 - rand.Float32()*10, Z: 0}
	} else {
		ps.Particles[i].Velocity = rl.Vector3{X: rand.Float32()*2 - 1, Y: -2 - rand.Float32()*2, Z: rand.Float32()*2 - 1}
	}
	ps.Particles[i].Alpha = 1.0
	ps.Particles[i].Active = true
}

func (ps *ParticleSystem) Update(dt float32, camPos rl.Vector3) {
	for i := 0; i < ps.MaxParticles; i++ {
		if !ps.Particles[i].Active {
			continue
		}

		ps.Particles[i].Position.X += ps.Particles[i].Velocity.X * dt
		ps.Particles[i].Position.Y += ps.Particles[i].Velocity.Y * dt
		ps.Particles[i].Position.Z += ps.Particles[i].Velocity.Z * dt

		// Reposiciona se sair do campo de visão ou bater no chão
		if ps.Particles[i].Position.Y < -10 {
			ps.resetParticle(i)
			// Move para perto da câmera
			ps.Particles[i].Position.X += camPos.X
			ps.Particles[i].Position.Z += camPos.Z
		}
	}
}

func (ps *ParticleSystem) Draw() {
	if ps.Type == WeatherNone {
		return
	}

	for i := 0; i < ps.MaxParticles; i++ {
		if ps.Type == WeatherRain {
			rl.DrawLine3D(ps.Particles[i].Position,
				rl.Vector3{X: ps.Particles[i].Position.X, Y: ps.Particles[i].Position.Y + 0.5, Z: ps.Particles[i].Position.Z},
				rl.NewColor(100, 150, 255, 150))
		} else {
			rl.DrawCube(ps.Particles[i].Position, 0.1, 0.1, 0.1, rl.NewColor(255, 255, 255, 200))
		}
	}
}
