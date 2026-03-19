package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

// Breaker implements the circuit breaker pattern.
type Breaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	threshold   int
	timeout     time.Duration
	halfOpenMax int
	lastFailure time.Time
	halfOpenCnt int
}

// NewBreaker creates a new circuit breaker.
func NewBreaker(threshold int, timeout time.Duration, halfOpenMax int) *Breaker {
	return &Breaker{
		state:       Closed,
		threshold:   threshold,
		timeout:     timeout,
		halfOpenMax: halfOpenMax,
	}
}

// Allow checks if the request should be allowed.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == Open {
		if time.Since(b.lastFailure) > b.timeout {
			b.state = HalfOpen
			b.halfOpenCnt = 0
			return true
		}
		return false
	}

	if b.state == HalfOpen {
		if b.halfOpenCnt >= b.halfOpenMax {
			return false
		}
		b.halfOpenCnt++
		return true
	}

	return true
}

// RecordSuccess records a successful request.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == HalfOpen {
		b.state = Closed
		b.failures = 0
	} else if b.state == Closed {
		b.failures = 0
	}
}

// RecordFailure records a failed request.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	b.lastFailure = time.Now()

	if b.state == Closed {
		if b.failures >= b.threshold {
			b.state = Open
		}
	} else if b.state == HalfOpen {
		b.state = Open
	}
}

// State returns the current circuit breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
