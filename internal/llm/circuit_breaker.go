package llm

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures
type CircuitBreaker struct {
	name            string
	maxFailures     int
	resetTimeout    time.Duration
	mu              sync.RWMutex
	state           CircuitBreakerState
	failures        int
	lastFailTime    time.Time
	halfOpenSuccess int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Execute executes the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker '%s' is %s", cb.name, cb.getStateString())
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canExecute determines if the operation should be allowed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should try half-open
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenSuccess = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()

		switch cb.state {
		case StateClosed:
			if cb.failures >= cb.maxFailures {
				cb.state = StateOpen
				cb.lastFailTime = time.Now()
			}
		case StateHalfOpen:
			cb.state = StateOpen
		}
	} else {
		// Success
		switch cb.state {
		case StateHalfOpen:
			cb.halfOpenSuccess++
			if cb.halfOpenSuccess >= 3 { // Need 3 consecutive successes to close
				cb.state = StateClosed
				cb.failures = 0
			}
		case StateOpen:
			// Shouldn't happen, but handle gracefully
			cb.state = StateClosed
			cb.failures = 0
		case StateClosed:
			cb.failures = 0
		}
	}
}

// getStateString returns a string representation of the current state
func (cb *CircuitBreaker) getStateString() string {
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns statistics about the circuit breaker
func (cb *CircuitBreaker) Stats() (state string, failures int, lastFailTime time.Time) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.getStateString(), cb.failures, cb.lastFailTime
}
