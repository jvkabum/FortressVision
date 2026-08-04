package render

import (
	"kaijuengine.com/engine"
	"kaijuengine.com/matrix"
	"fmt"
)

// SetupDiagnostics injeta objetos de depuração na cena para validação visual.
func SetupDiagnostics(host *engine.Host) {
	host.RunOnMainThread(func() {
		// Posicionar a esfera vermelha próxima ao ponto inicial da visão do DF
		// Isso ajuda o usuário a se localizar antes do primeiro chunk carregar.
		CreateRedBall(host, matrix.NewVec3(103, 94, -115))
		fmt.Println("[Diagnostics] 🔴 Esfera de diagnóstico ativa em (103, 94, -115)")
	})
}
