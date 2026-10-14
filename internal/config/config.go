package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	DefaultRate     float64       // Tokens per second
	DefaultBurst    int           // Max capacity
	CleanupInterval time.Duration // Interval to clean up stale buckets
	ClientTTL       time.Duration // Time after which inactive client is removed
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DefaultRate:     getEnvFloat("RATE_LIMIT", 5.0),
		DefaultBurst:    getEnvInt("BURST_LIMIT", 10),
		CleanupInterval: time.Minute * 2,
		ClientTTL:       time.Minute * 10,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}
