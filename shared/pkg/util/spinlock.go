package util

import (
	"runtime"
	"sync/atomic"
)

// SpinLock provê uma primitiva de exclusão mútua onde a thread espera em um loop.
// Útil para seções críticas extremamente curtas onde o custo de context switch do Mutex é maior que o spin.
type SpinLock struct {
	state int32
}

// Lock adquire o bloqueio.
func (s *SpinLock) Lock() {
	for !atomic.CompareAndSwapInt32(&s.state, 0, 1) {
		runtime.Gosched() // Permite que outras goroutines rodem sem travar o processador 100%
	}
}

// Unlock libera o bloqueio.
func (s *SpinLock) Unlock() {
	atomic.StoreInt32(&s.state, 0)
}

// TryLock tenta adquirir o bloqueio sem esperar.
func (s *SpinLock) TryLock() bool {
	return atomic.CompareAndSwapInt32(&s.state, 0, 1)
}
