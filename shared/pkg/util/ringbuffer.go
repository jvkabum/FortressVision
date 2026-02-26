package util

import (
	"errors"
	"sync/atomic"
)

// RingBuffer é um buffer circular de alta performance e baixa alocação (semelhante ao Disruptor).
// Adaptado do RingBuffer.cs do Armok Vision.
type RingBuffer[T any] struct {
	entries    []T
	mask       uint64
	producerID uint64
	consumerID uint64
}

// NewRingBuffer cria um novo buffer circular com a capacidade dada (será arredondada para potência de 2).
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	actualCap := nextPowerOfTwo(capacity)
	return &RingBuffer[T]{
		entries: make([]T, actualCap),
		mask:    uint64(actualCap - 1),
	}
}

// Enqueue adiciona um item ao buffer. Retorna erro se estiver cheio.
func (r *RingBuffer[T]) Enqueue(item T) error {
	next := atomic.LoadUint64(&r.producerID)
	consumer := atomic.LoadUint64(&r.consumerID)

	if next-consumer >= uint64(len(r.entries)) {
		return errors.New("buffer circular cheio")
	}

	r.entries[next&r.mask] = item
	atomic.AddUint64(&r.producerID, 1)
	return nil
}

// Dequeue remove um item do buffer. Retorna erro se estiver vazio.
func (r *RingBuffer[T]) Dequeue() (T, error) {
	var zero T
	consumer := atomic.LoadUint64(&r.consumerID)
	producer := atomic.LoadUint64(&r.producerID)

	if consumer >= producer {
		return zero, errors.New("buffer circular vazio")
	}

	item := r.entries[consumer&r.mask]
	// Opcional: r.entries[consumer&r.mask] = zero (para evitar vazamento de memória se T for ponteiro)
	atomic.AddUint64(&r.consumerID, 1)
	return item, nil
}

func nextPowerOfTwo(x int) int {
	res := 2
	for res < x {
		res <<= 1
	}
	return res
}
