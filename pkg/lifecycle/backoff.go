package lifecycle

import (
	"math/rand"
	"time"
)

// Backoff implements exponential backoff with jitter.
type Backoff struct {
	initial time.Duration
	max     time.Duration
	current time.Duration
}

// NewBackoff creates a new backoff with the given initial and max durations.
func NewBackoff(initial, max time.Duration) *Backoff {
	return &Backoff{
		initial: initial,
		max:     max,
		current: initial,
	}
}

// Sleep sleeps for the current backoff duration and increases it.
func (b *Backoff) Sleep() {
	// Add jitter: ±20%
	jitter := float64(b.current) * 0.2 * (rand.Float64()*2 - 1)
	sleep := time.Duration(float64(b.current) + jitter)

	time.Sleep(sleep)

	// Increase for next time
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
}

// Reset resets the backoff to the initial duration.
func (b *Backoff) Reset() {
	b.current = b.initial
}

// Current returns the current backoff duration.
func (b *Backoff) Current() time.Duration {
	return b.current
}
