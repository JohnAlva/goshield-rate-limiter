package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_BasicAllow(t *testing.T) {
	tb := NewTokenBucket(1.0, 3)

	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	if tb.Allow() {
		t.Fatal("expected request 4 to be rejected after burst exhaustion")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	// 10 tokens per second, burst 2
	tb := NewTokenBucket(10.0, 2)

	tb.Allow()
	tb.Allow()

	if tb.Allow() {
		t.Fatal("expected bucket to be empty")
	}

	// Wait 150ms -> should refill at least 1 token
	time.Sleep(150 * time.Millisecond)

	if !tb.Allow() {
		t.Fatal("expected at least 1 token after refill time")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager(100.0, 50, time.Minute, time.Minute)
	var wg sync.WaitGroup

	numGoroutines := 20
	requestsPerGoroutine := 25

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				m.Allow("client-concurrent")
			}
		}(i)
	}

	wg.Wait()

	stats := m.GetStats()
	total := stats.TotalAllowed + stats.TotalBlocked
	expected := uint64(numGoroutines * requestsPerGoroutine)

	if total != expected {
		t.Fatalf("expected %d total processed requests, got %d", expected, total)
	}
}
