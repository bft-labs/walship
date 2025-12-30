package app

import (
	"math/rand"
	"time"
)

// Default backoff configuration values.
const (
	DefaultBackoffInitial = 500 * time.Millisecond
	DefaultBackoffMax     = 10 * time.Second
)

// backoff implements exponential backoff with jitter.
type backoff struct {
	initial time.Duration
	max     time.Duration
	current time.Duration
}

// newBackoff creates a new backoff with the given initial and max durations.
func newBackoff(initial, max time.Duration) *backoff {
	return &backoff{
		initial: initial,
		max:     max,
		current: initial,
	}
}

// Sleep sleeps for the current backoff duration and increases it.
func (b *backoff) Sleep() {
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
func (b *backoff) Reset() {
	b.current = b.initial
}

// Current returns the current backoff duration.
func (b *backoff) Current() time.Duration {
	return b.current
}
