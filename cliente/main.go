package main

import (
	"FortressVision/cliente/internal/core"
	"kaijuengine.com/engine"
	"kaijuengine.com/engine/systems/console"
)

// ConstructionApp é a orquestração principal do cliente FortressVision.
type ConstructionApp struct {
	core.App
	controller *core.AppController
}

// Launch é o ponto de entrada da lógica da aplicação na engine Kaiju.
func (a *ConstructionApp) Launch(host *engine.Host) {
	a.App.Launch(host)
	console.For(host)

	// Inicializar o Controlador Central (Bootsrapping de todos os subsistemas)
	a.controller = core.NewAppController(host)

	// Única responsabilidade do main: delegar o loop de atualização ao controlador.
	host.Updater.AddUpdate(func(dt float64) {
		a.controller.Update(dt)
	})
}

func main() {
	core.Start(&ConstructionApp{})
}
