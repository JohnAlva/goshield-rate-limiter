package limiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// TokenBucket implements the thread-safe token bucket algorithm.
type TokenBucket struct {
	mu           sync.Mutex
	rate         float64   // tokens added per second
	capacity     float64   // maximum tokens bucket can hold
	tokens       float64   // current available tokens
	lastRefill   time.Time // timestamp of last token refill
	lastAccessed time.Time // timestamp of last request
}

// NewTokenBucket creates a new token bucket instance.
func NewTokenBucket(rate float64, capacity int) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		rate:         rate,
		capacity:     float64(capacity),
		tokens:       float64(capacity),
		lastRefill:   now,
		lastAccessed: now,
	}
}

// Allow checks if a single token can be consumed.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if n tokens can be consumed.
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	tb.lastAccessed = now

	// Calculate tokens to refill based on elapsed time
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	// Check if enough tokens are available
	required := float64(n)
	if tb.tokens >= required {
		tb.tokens -= required
		return true
	}

	return false
}

// Remaining returns the current number of available tokens.
func (tb *TokenBucket) Remaining() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return int(tb.tokens)
}

// Stats holds observability metrics for the rate limiter.
type Stats struct {
	TotalAllowed uint64 `json:"total_allowed"`
	TotalBlocked uint64 `json:"total_blocked"`
	ActiveKeys   int    `json:"active_keys"`
}

// Manager coordinates rate limiters for multiple clients/IPs.
type Manager struct {
	mu              sync.RWMutex
	buckets         map[string]*TokenBucket
	defaultRate     float64
	defaultCapacity int
	clientTTL       time.Duration
	totalAllowed    uint64
	totalBlocked    uint64
}

// NewManager creates a manager for multi-tenant rate limiting.
func NewManager(rate float64, capacity int, cleanupInterval, clientTTL time.Duration) *Manager {
	m := &Manager{
		buckets:         make(map[string]*TokenBucket),
		defaultRate:     rate,
		defaultCapacity: capacity,
		clientTTL:       clientTTL,
	}

	go m.cleanupLoop(cleanupInterval)
	return m
}

// Allow evaluates a request for a specific client key.
func (m *Manager) Allow(key string) bool {
	bucket := m.getBucket(key)
	if bucket.Allow() {
		atomic.AddUint64(&m.totalAllowed, 1)
		return true
	}
	atomic.AddUint64(&m.totalBlocked, 1)
	return false
}

// Remaining returns available tokens for a specific client key.
func (m *Manager) Remaining(key string) int {
	return m.getBucket(key).Remaining()
}

func (m *Manager) getBucket(key string) *TokenBucket {
	m.mu.RLock()
	bucket, exists := m.buckets[key]
	m.mu.RUnlock()

	if exists {
		return bucket
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check after acquiring write lock
	if bucket, exists = m.buckets[key]; exists {
		return bucket
	}

	bucket = NewTokenBucket(m.defaultRate, m.defaultCapacity)
	m.buckets[key] = bucket
	return bucket
}

func (m *Manager) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		m.cleanup()
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, bucket := range m.buckets {
		bucket.mu.Lock()
		idle := now.Sub(bucket.lastAccessed)
		bucket.mu.Unlock()

		if idle > m.clientTTL {
			delete(m.buckets, key)
		}
	}
}

// GetStats returns current system metrics.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	active := len(m.buckets)
	m.mu.RUnlock()

	return Stats{
		TotalAllowed: atomic.LoadUint64(&m.totalAllowed),
		TotalBlocked: atomic.LoadUint64(&m.totalBlocked),
		ActiveKeys:   active,
	}
}
