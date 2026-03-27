package mesher

import (
	"FortressVision/shared/mapdata"
	"log"
	"sync"
)

// Manager coordena a geração de geometria baseada nas mudanças do mundo.
type Manager struct {
	mu            sync.RWMutex
	meshes        map[string]*ChunkMesh
	pendingChunks chan *mapdata.Chunk
}

// NewManager inicializa o gerenciador de alvenaria.
func NewManager() *Manager {
	m := &Manager{
		meshes:        make(map[string]*ChunkMesh),
		pendingChunks: make(chan *mapdata.Chunk, 100),
	}
	
	// Worker pool para meshing asíncrono
	go m.worker()
	
	return m
}

// RequestMeshUpdate solicita que um chunk seja transformado em 3D.
func (m *Manager) RequestMeshUpdate(chunk *mapdata.Chunk) {
	if chunk == nil {
		return
	}
	m.pendingChunks <- chunk
}

func (m *Manager) worker() {
	for chunk := range m.pendingChunks {
		// Gerar a malha (Greedy)
		mesh := GenerateChunkMesh(chunk)
		
		m.mu.Lock()
		key := chunk.Origin.String()
		m.meshes[key] = mesh
		m.mu.Unlock()
		
		log.Printf("🧱 [Mesher] Geometria gerada para chunk %v (%d materiais)", chunk.Origin, len(mesh.SubMeshes))
	}
}

// GetMeshes retorna as malhas atuais para o renderizador.
func (m *Manager) GetMeshes() []*ChunkMesh {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	list := make([]*ChunkMesh, 0, len(m.meshes))
	for _, mesh := range m.meshes {
		list = append(list, mesh)
	}
	return list
}
