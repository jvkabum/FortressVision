package meshing

import (
	"FortressVision/shared/util"
	"sync"
)

// ResultStore armazena os resultados de meshing na RAM para evitar re-processamento.
type ResultStore struct {
	mu      sync.RWMutex
	results map[util.DFCoord]Result
}

// NewResultStore cria um novo repositório de resultados.
func NewResultStore() *ResultStore {
	return &ResultStore{
		results: make(map[util.DFCoord]Result),
	}
}

// Get retorna um resultado se ele existir e for compatível com o MTime informado.
func (s *ResultStore) Get(coord util.DFCoord, mtime int64) (Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, ok := s.results[coord]
	if ok && res.MTime == mtime {
		// Retornamos um clone para evitar que modificações externas afetem o cache
		return res.Clone(), true
	}
	return Result{}, false
}

// Store salva um resultado no repositório.
func (s *ResultStore) Store(res Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Guardamos um clone para garantir que o cache seja imutável
	s.results[res.Origin] = res.Clone()
}

// Clear limpa todo o cache de resultados.
func (s *ResultStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = make(map[util.DFCoord]Result)
}

// Clone realiza uma cópia profunda de um Result.
func (r Result) Clone() Result {
	newRes := Result{
		Origin:             r.Origin,
		MTime:              r.MTime,
		Terreno:            r.Terreno.Clone(),
		Liquidos:           r.Liquidos.Clone(),
		MaterialGeometries: make(map[string]GeometryData),
		ModelInstances:     make([]ModelInstance, len(r.ModelInstances)),
	}

	for k, v := range r.MaterialGeometries {
		newRes.MaterialGeometries[k] = v.Clone()
	}
	copy(newRes.ModelInstances, r.ModelInstances)

	return newRes
}
