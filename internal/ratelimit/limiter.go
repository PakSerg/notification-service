// Package ratelimit implements a per-key token bucket used to throttle
// inbound requests, e.g. by client IP.
package ratelimit

import (
	"sync"
	"time"
)

// staleAfter bounds how long an idle bucket is kept around, so a stream of
// distinct keys (e.g. spoofed IPs) can't grow the map without limit.
const staleAfter = 10 * time.Minute

// Limiter grants each key a token bucket that refills at rate tokens/sec up
// to burst capacity. Keys are created lazily on first use.
type Limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	calls   uint64
	now     func() time.Time
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// New creates a Limiter allowing `rate` requests per second per key, with
// bursts up to `burst` requests.
func New(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:    rate,
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// Allow reports whether a request for the given key may proceed. It
// consumes a token from the key's bucket when it does.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.evictStaleLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	} else {
		b.tokens += now.Sub(b.lastSeen).Seconds() * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastSeen = now
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// evictStaleLocked periodically drops buckets that haven't been touched in
// a while. It runs every so many calls rather than on every call, since a
// full map scan on the hot path would defeat the point of rate limiting.
func (l *Limiter) evictStaleLocked(now time.Time) {
	l.calls++
	if l.calls%1024 != 0 {
		return
	}
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > staleAfter {
			delete(l.buckets, k)
		}
	}
}
