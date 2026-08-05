package mesher

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"log"
	"sync"
)

// Manager coordena a geração de geometria baseada nas mudanças do mundo.
type Manager struct {
	mu            sync.RWMutex
	meshes        map[string]*ChunkMesh
	pendingChunks chan string
	pending       map[string]*mapdata.Chunk
	queued        map[string]bool
	removed       map[string]bool

	OnMeshGenerated func(mesh *ChunkMesh)
}

// NewManager inicializa o gerenciador de alvenaria.
func NewManager() *Manager {
	m := &Manager{
		meshes:        make(map[string]*ChunkMesh),
		pendingChunks: make(chan string, 100),
		pending:       make(map[string]*mapdata.Chunk),
		queued:        make(map[string]bool),
		removed:       make(map[string]bool),
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
	// O scanner pode atualizar o mesmo chunk enquanto o worker está lendo a
	// geometria. Enfileirar uma cópia elimina essa corrida e mantém cada mesh
	// coerente com um único instante do DFHack.
	chunk = chunk.Clone()
	key := chunk.Origin.String()
	m.mu.Lock()
	delete(m.removed, key)
	m.pending[key] = chunk
	shouldQueue := !m.queued[key]
	if shouldQueue {
		m.queued[key] = true
	}
	m.mu.Unlock()
	if shouldQueue {
		m.pendingChunks <- key
	}
}

// RemoveChunk cancela uma malha pendente e impede que o worker publique um
// chunk que o streaming já classificou como ar.
func (m *Manager) RemoveChunk(origin util.DFCoord) {
	key := origin.String()
	m.mu.Lock()
	delete(m.pending, key)
	delete(m.meshes, key)
	m.removed[key] = true
	m.mu.Unlock()
}

func (m *Manager) worker() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Erro fatal no Worker do Mesher (Thread morta salva): %v", r)
			// Reinicia o worker em caso de pânico para não travar o canal pendente!
			go m.worker()
		}
	}()

	for key := range m.pendingChunks {
		m.mu.Lock()
		chunk := m.pending[key]
		delete(m.pending, key)
		m.mu.Unlock()
		if chunk == nil {
			m.mu.Lock()
			delete(m.queued, key)
			m.mu.Unlock()
			continue
		}

		// Gerar a malha (Greedy)
		mesh := GenerateChunkMesh(chunk)

		m.mu.Lock()
		meshKey := chunk.Origin.String()
		removed := m.removed[meshKey]
		if !removed {
			m.meshes[meshKey] = mesh
		}
		cb := m.OnMeshGenerated
		m.mu.Unlock()

		if cb != nil && !removed {
			cb(mesh)
		}

		m.mu.Lock()
		_, needsRetry := m.pending[key]
		if !needsRetry {
			delete(m.queued, key)
		}
		m.mu.Unlock()
		if needsRetry {
			m.pendingChunks <- key
		}

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
