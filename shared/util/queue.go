package util

import "sync"

// UniqueQueue é uma fila thread-safe que garante elementos únicos por chave.
// Replica a UniqueQueue do Armok Vision usada para enfileirar blocos para meshing.
type UniqueQueue[K comparable, V any] struct {
	mu      sync.Mutex
	items   []entry[K, V]
	present map[K]bool
}

type entry[K comparable, V any] struct {
	Key   K
	Value V
}

// NewUniqueQueue cria uma nova UniqueQueue.
func NewUniqueQueue[K comparable, V any]() *UniqueQueue[K, V] {
	return &UniqueQueue[K, V]{
		items:   make([]entry[K, V], 0, 64),
		present: make(map[K]bool),
	}
}

// Enqueue adiciona um item se a chave ainda não existir na fila.
// Se a chave já existir, o valor é atualizado.
// Retorna true se foi adicionado (novo), false se foi atualizado.
func (q *UniqueQueue[K, V]) Enqueue(key K, value V) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.present[key] {
		// Atualizar valor existente
		for i := range q.items {
			if q.items[i].Key == key {
				q.items[i].Value = value
				break
			}
		}
		return false
	}

	q.items = append(q.items, entry[K, V]{Key: key, Value: value})
	q.present[key] = true
	return true
}

// Dequeue remove e retorna o primeiro item da fila.
// Retorna o valor, a chave e true se havia item; zero values e false se vazia.
func (q *UniqueQueue[K, V]) Dequeue() (K, V, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	e := q.items[0]
	q.items = q.items[1:]
	delete(q.present, e.Key)
	return e.Key, e.Value, true
}

// Len retorna o número de items na fila.
func (q *UniqueQueue[K, V]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Clear limpa a fila.
func (q *UniqueQueue[K, V]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
	q.present = make(map[K]bool)
}

// Contains verifica se uma chave está na fila.
func (q *UniqueQueue[K, V]) Contains(key K) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.present[key]
}

// ThreadSafeQueue é uma fila simples thread-safe (sem unicidade).
type ThreadSafeQueue[T any] struct {
	mu    sync.Mutex
	items []T
}

// NewThreadSafeQueue cria uma nova fila thread-safe.
func NewThreadSafeQueue[T any]() *ThreadSafeQueue[T] {
	return &ThreadSafeQueue[T]{
		items: make([]T, 0, 64),
	}
}

// Push adiciona um item ao fim da fila.
func (q *ThreadSafeQueue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

// Pop remove e retorna o primeiro item. Retorna false se vazia.
func (q *ThreadSafeQueue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// Len retorna o tamanho da fila.
func (q *ThreadSafeQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
