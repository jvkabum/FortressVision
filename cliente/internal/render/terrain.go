package render

import (
	"FortressVision/cliente/internal/mesher"
	"fmt"
	"sync"
	"time"

	"kaijuengine.com/engine"
	"kaijuengine.com/rendering"
)

// TerrainRenderer gerencia a exibição do terreno 3D na engine Kaiju.
type TerrainRenderer struct {
	host           *engine.Host
	mu             sync.Mutex
	activeDrawings map[string][]rendering.Drawing
	activeMeshKeys map[string]string
	meshingMgr     *mesher.Manager
}

// NewTerrainRenderer inicializa um novo renderizador de terreno desacoplado.
func NewTerrainRenderer(host *engine.Host, meshingMgr *mesher.Manager) *TerrainRenderer {
	tr := &TerrainRenderer{
		host:           host,
		activeDrawings: make(map[string][]rendering.Drawing),
		activeMeshKeys: make(map[string]string),
		meshingMgr:     meshingMgr,
	}

	// Vincular evento de geração de malha
	tr.meshingMgr.OnMeshGenerated = tr.onMeshGenerated

	return tr
}

// onMeshGenerated lida com novas geometrias vindas do mesher assíncrono.
func (tr *TerrainRenderer) onMeshGenerated(mesh *mesher.ChunkMesh) {
	tr.host.RunOnMainThread(func() {
		tr.mu.Lock()
		defer tr.mu.Unlock()

		key := mesh.Origin.String()

		// 1. Destruir instâncias de shader antigas (VAO/Buffers)
		if oldDraws, ok := tr.activeDrawings[key]; ok {
			for _, d := range oldDraws {
				if d.ShaderData != nil {
					d.ShaderData.Destroy()
				}
			}
		}

		// 2. Gerar nova chave única para este chunk (Evita conflitos de driver)
		newMeshKey := fmt.Sprintf("chunk_%s_%d", key, time.Now().UnixNano())
		
		// 3. Criar e armazenar novos desenhos
		newDraws := CreateChunkDrawings(tr.host, mesh, newMeshKey)
		tr.activeDrawings[key] = newDraws

		// 4. Limpeza do Cache de Malhas da GPU (Soft Handover)
		if oldKey, ok := tr.activeMeshKeys[key]; ok {
			tr.host.MeshCache().RemoveMesh(oldKey)
		}
		tr.activeMeshKeys[key] = newMeshKey
	})
}

// Render submete todos os desenhos ativos para o pipeline da KaijuEngine.
// Deve ser chamado dentro do loop AddUpdate principal.
func (tr *TerrainRenderer) Render() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	for _, draws := range tr.activeDrawings {
		for _, d := range draws {
			tr.host.Drawings.AddDrawing(d)
		}
	}
}
