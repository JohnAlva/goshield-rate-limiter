package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/JohnAlva/goshield-rate-limiter/internal/limiter"
)

// RateLimiter returns an HTTP middleware that applies rate limiting based on client IP or API Key.
func RateLimiter(mgr *limiter.Manager, burstLimit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientKey := extractClientKey(r)

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", burstLimit))

			if !mgr.Allow(clientKey) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":       "Too Many Requests",
					"message":     "Rate limit exceeded. Please back off before retrying.",
					"client":      clientKey,
					"status_code": http.StatusTooManyRequests,
				})
				return
			}

			remaining := mgr.Remaining(clientKey)
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			next.ServeHTTP(w, r)
		})
	}
}

func extractClientKey(r *http.Request) string {
	// 1. Check for API Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}

	// 2. Check X-Forwarded-For header if behind reverse proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return "ip:" + strings.TrimSpace(ips[0])
	}

	// 3. Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + host
}
