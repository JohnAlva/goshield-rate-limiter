package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JohnAlva/goshield-rate-limiter/internal/config"
	"github.com/JohnAlva/goshield-rate-limiter/internal/limiter"
	"github.com/JohnAlva/goshield-rate-limiter/internal/middleware"
)

func main() {
	cfg := config.Load()

	// Initialize thread-safe rate limiter manager
	mgr := limiter.NewManager(cfg.DefaultRate, cfg.DefaultBurst, cfg.CleanupInterval, cfg.ClientTTL)

	mux := http.NewServeMux()

	// Public Health Check (unthrottled)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "GoShield Rate Limiter",
			"version":   "1.0.0",
		})
	})

	// Protected API Routes
	apiMux := http.NewServeMux()

	apiMux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "pong",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	apiMux.HandleFunc("POST /api/v1/echo", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"echo":      payload,
			"processed": time.Now().UTC().Format(time.RFC3339),
		})
	})

	apiMux.HandleFunc("GET /api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mgr.GetStats())
	})

	// Wrap API routes with RateLimiter middleware
	mux.Handle("/api/", middleware.RateLimiter(mgr, cfg.DefaultBurst)(apiMux))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handling
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("🛡️ GoShield Gateway listening on http://0.0.0.0:%s", cfg.Port)
		log.Printf("⚙️ Configuration: Rate=%.1f req/s, Burst=%d tokens", cfg.DefaultRate, cfg.DefaultBurst)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case sig := <-shutdown:
		log.Printf("Signal %v received, initiating graceful shutdown...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed, forcing close: %v", err)
			_ = server.Close()
		}
		log.Println("GoShield stopped safely.")
	}
}
