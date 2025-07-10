package circuit

import (
	"sync"
	"sync/atomic"
	"time"
)

type CircuitBreaker struct {
	failures    int64
	lastFailure time.Time
	threshold   int64
	timeout     time.Duration
	mu          sync.RWMutex
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		threshold: 5,
		timeout:   30 * time.Second,
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if atomic.LoadInt64(&cb.failures) >= cb.threshold {
		return time.Since(cb.lastFailure) < cb.timeout
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	atomic.StoreInt64(&cb.failures, 0)
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.failures, 1)
	cb.lastFailure = time.Now()
}