package agent

import (
	"math/rand"
	"time"
)

type backoff struct {
	base time.Duration
	max  time.Duration
	cur  time.Duration
}

func newBackoff(base, max time.Duration) *backoff { return &backoff{base: base, max: max} }

func (b *backoff) Sleep() {
	if b.cur <= 0 {
		b.cur = b.base
	} else {
		b.cur *= 2
		if b.cur > b.max {
			b.cur = b.max
		}
	}
	// jitter ~ +/-20%
	j := 0.8 + 0.4*rand.Float64()
	time.Sleep(time.Duration(float64(b.cur) * j))
}

func (b *backoff) Reset() { b.cur = 0 }
