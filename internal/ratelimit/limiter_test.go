package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinBurst(t *testing.T) {
	l := New(1, 3)

	for i := 0; i < 3; i++ {
		if !l.Allow("client") {
			t.Fatalf("request %d: expected allowed within burst", i)
		}
	}

	if l.Allow("client") {
		t.Fatal("expected request beyond burst to be denied")
	}
}

func TestAllowRefillsOverTime(t *testing.T) {
	l := New(1, 1)
	now := time.Now()
	l.now = func() time.Time { return now }

	if !l.Allow("client") {
		t.Fatal("expected first request to be allowed")
	}
	if l.Allow("client") {
		t.Fatal("expected immediate second request to be denied")
	}

	now = now.Add(time.Second)
	if !l.Allow("client") {
		t.Fatal("expected request to be allowed after refill")
	}
}

func TestAllowKeysAreIndependent(t *testing.T) {
	l := New(1, 1)

	if !l.Allow("a") {
		t.Fatal("expected first request for key a to be allowed")
	}
	if !l.Allow("b") {
		t.Fatal("expected first request for key b to be allowed")
	}
	if l.Allow("a") {
		t.Fatal("expected second request for key a to be denied")
	}
}

func TestEvictStaleBuckets(t *testing.T) {
	l := New(1, 1)
	now := time.Now()
	l.now = func() time.Time { return now }

	l.Allow("client")

	now = now.Add(staleAfter + time.Second)
	for i := uint64(0); i < 1024; i++ {
		l.Allow("other")
	}

	l.mu.Lock()
	_, stillPresent := l.buckets["client"]
	l.mu.Unlock()

	if stillPresent {
		t.Fatal("expected stale bucket to be evicted")
	}
}
