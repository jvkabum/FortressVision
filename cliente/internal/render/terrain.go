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

	// Vincular evento de geração de malha.
	tr.meshingMgr.OnMeshGenerated = tr.onMeshGenerated

	return tr
}

// onMeshGenerated lida com novas geometrias vindas do mesher assíncrono.
func (tr *TerrainRenderer) onMeshGenerated(mesh *mesher.ChunkMesh) {
	tr.host.RunOnMainThread(func() {
		tr.mu.Lock()
		defer tr.mu.Unlock()

		key := mesh.Origin.String()

		// Destruir instâncias de shader antigas (VAO/Buffers).
		if oldDraws, ok := tr.activeDrawings[key]; ok {
			for _, d := range oldDraws {
				if d.ShaderData != nil {
					d.ShaderData.Destroy()
				}
			}
		}

		newMeshKey := fmt.Sprintf("chunk_%s_%d", key, time.Now().UnixNano())
		newDraws := CreateChunkDrawings(tr.host, mesh, newMeshKey)
		tr.activeDrawings[key] = newDraws
		tr.host.Drawings.AddDrawings(newDraws)

		if oldKey, ok := tr.activeMeshKeys[key]; ok {
			tr.host.MeshCache().RemoveMesh(oldKey)
		}
		tr.activeMeshKeys[key] = newMeshKey
	})
}

// Render mantém a API do controlador; os desenhos já são registrados quando a
// malha termina de ser gerada e permanecem persistentes na KaijuEngine.
func (tr *TerrainRenderer) Render() {
}
