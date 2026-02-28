package render

import (
	"FortressVision/cliente/internal/meshing"
	"math"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// PropBatch agrupa instâncias do mesmo modelo e material para desenho instanciado.
type PropBatch struct {
	ModelName   string
	TextureName string
	Mesh        rl.Mesh
	Material    rl.Material
	Transforms  []rl.Matrix
	Visible     []rl.Matrix // Buffer reaproveitado para instâncias visíveis (Fase 33)
}

// Draw renderiza as instâncias que passaram pelo culling.
func (b *PropBatch) Draw() {
	count := len(b.Visible)
	if count == 0 {
		return
	}

	// 1 draw call para todas as instâncias visíveis deste tipo
	// Nota: No raylib-go moderno, o último argumento é int
	rl.DrawMeshInstanced(b.Mesh, b.Material, b.Visible, count)
}

// PropManager coordena múltiplos lotes de instanciamento com suporte a Culling.
type PropManager struct {
	Batches map[string]*PropBatch
}

func NewPropManager() *PropManager {
	return &PropManager{
		Batches: make(map[string]*PropBatch),
	}
}

// Clear reseta os buffers sem desalocar memória (zero garbage).
func (pm *PropManager) Clear() {
	for _, b := range pm.Batches {
		b.Transforms = b.Transforms[:0]
		b.Visible = b.Visible[:0]
	}
}

// AddInstance adiciona uma instância com a ordem de transformação CORRETA (T * R * S).
func (pm *PropManager) AddInstance(inst meshing.ModelInstance, mesh rl.Mesh, material rl.Material) {
	key := inst.ModelName + ":" + inst.TextureName
	batch, ok := pm.Batches[key]
	if !ok {
		batch = &PropBatch{
			ModelName:   inst.ModelName,
			TextureName: inst.TextureName,
			Mesh:        mesh,
			Material:    material,
			Transforms:  make([]rl.Matrix, 0, 2048),
			Visible:     make([]rl.Matrix, 0, 2048),
		}
		pm.Batches[key] = batch
	}

	// 1. Escala (S)
	if inst.Scale == 0 {
		inst.Scale = 1.0
	}
	scaleMat := rl.MatrixScale(inst.Scale, inst.Scale, inst.Scale)
	// 2. Rotação (R) - inst.Rotation deveria vir em graus (0-360) do JSON, Raylib MatrixRotateY aceita Radianos
	rotMat := rl.MatrixRotateY(inst.Rotation * (math.Pi / 180.0))
	// 3. Translação (T)
	transMat := rl.MatrixTranslate(inst.Position[0], inst.Position[1], inst.Position[2])

	// Ordem CORRETA (T * R * S) para Raylib/OpenGL:
	// 1. Scale local -> 2. Rotate local -> 3. Translate to World
	// No Raylib MatrixMultiply(A, B) é A * B. Queremos T * R * S.
	matrix := rl.MatrixMultiply(rotMat, scaleMat)
	matrix = rl.MatrixMultiply(transMat, matrix)

	batch.Transforms = append(batch.Transforms, matrix)
}

// DrawAll executa Frustum Culling e desenha as instâncias visíveis.
func (pm *PropManager) DrawAll(cam rl.Camera3D) {
	// camDir := rl.Vector3Normalize(rl.Vector3Subtract(cam.Target, cam.Position))

	for _, b := range pm.Batches {
		if len(b.Transforms) == 0 {
			continue
		}

		// Reuso do buffer visível para evitar alocações por frame
		b.Visible = b.Visible[:0]

		for _, m := range b.Transforms {
			// Extrair posição da matriz (M12, M13, M14 em row-major da raylib) - DESABILITADO
			// pos := rl.Vector3{X: m.M12, Y: m.M13, Z: m.M14}

			// Frustum Culling manual via Dot Product - DESABILITADO PARA DEBUG
			// diff := rl.Vector3Subtract(pos, cam.Position)
			// distSq := diff.X*diff.X + diff.Y*diff.Y + diff.Z*diff.Z

			// Se estiver muito perto, sempre desenha (evita pop-in lateral)
			// if distSq < 225 { // 15 units
			b.Visible = append(b.Visible, m)
			// 	continue
			// }

			// Check de ângulo relaxado (dot product > 0.1 significa quase 90 graus de cada lado)
			// dirToPoint := rl.Vector3Normalize(diff)
			// dot := rl.Vector3DotProduct(camDir, dirToPoint)

			// if dot > 0.1 { // Muito mais permissivo
			// 	b.Visible = append(b.Visible, m)
			// }
		}

		b.Draw()
	}
}

// Helper seguro para obter o primeiro material de um modelo.
func getModelMaterial(model rl.Model) rl.Material {
	if model.MaterialCount > 0 {
		mats := unsafe.Slice(model.Materials, model.MaterialCount)
		return mats[0]
	}
	return rl.LoadMaterialDefault()
}
