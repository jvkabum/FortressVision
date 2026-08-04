package render

import (
	"FortressVision/cliente/internal/meshing"
	"FortressVision/shared/util"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// BlockModel representa a geometria renderizável de um bloco do mapa.
type BlockModel struct {
	Origin      util.DFCoord
	Model       rl.Model            // Geometria padrão (sem textura)
	MatModels   map[string]rl.Model // Modelos separados por textura (stone, grass, etc)
	LiquidModel rl.Model
	HasLiquid   bool
	Active      bool
	MTime       int64                   // Versão dos dados (para cache)
	Instances   []meshing.ModelInstance // Instâncias de modelos 3D neste bloco
}
