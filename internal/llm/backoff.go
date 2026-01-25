package llm

import (
	"math"
	"math/rand"
	"time"
)

// ExponentialBackoff provides exponential backoff with jitter for retry logic
type ExponentialBackoff struct {
	baseDelay    time.Duration
	maxDelay     time.Duration
	maxRetries   int
	multiplier   float64
	jitterFactor float64
}

// NewExponentialBackoff creates a new exponential backoff configuration
func NewExponentialBackoff(baseDelay, maxDelay time.Duration, maxRetries int) *ExponentialBackoff {
	return &ExponentialBackoff{
		baseDelay:    baseDelay,
		maxDelay:     maxDelay,
		maxRetries:   maxRetries,
		multiplier:   2.0,
		jitterFactor: 0.1,
	}
}

// Next returns the duration to wait for the next retry, or false if max retries exceeded
func (eb *ExponentialBackoff) Next(retry int) (time.Duration, bool) {
	if retry >= eb.maxRetries {
		return 0, false
	}

	// Calculate exponential delay
	delay := float64(eb.baseDelay) * math.Pow(eb.multiplier, float64(retry))

	// Add jitter to prevent thundering herd
	jitter := delay * eb.jitterFactor * (rand.Float64()*2 - 1) // +/- jitterFactor
	delay += jitter

	// Clamp to max delay
	if delay > float64(eb.maxDelay) {
		delay = float64(eb.maxDelay)
	}

	return time.Duration(delay), true
}

// RetryWithBackoff executes a function with exponential backoff retry
func (eb *ExponentialBackoff) RetryWithBackoff(fn func() error) error {
	var lastErr error

	for retry := 0; retry <= eb.maxRetries; retry++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if retry < eb.maxRetries {
			delay, shouldRetry := eb.Next(retry)
			if !shouldRetry {
				break
			}

			time.Sleep(delay)
		}
	}

	return lastErr
}
