package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

	APIKey string

	MaxAttempts            int
	BackoffBase            time.Duration
	BackoffMax             time.Duration
	DeliveryTimeout        time.Duration
	RecordTTL              time.Duration
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		APIKey:          getEnv("API_KEY", ""),
		MaxAttempts:     getEnvInt("MAX_ATTEMPTS", 4),
		BackoffBase:     time.Duration(getEnvFloat("BACKOFF_BASE_SECONDS", 2)) * time.Second,
		BackoffMax:      time.Duration(getEnvFloat("BACKOFF_MAX_SECONDS", 60)) * time.Second,
		DeliveryTimeout: time.Duration(getEnvFloat("DELIVERY_TIMEOUT_SECONDS", 10)) * time.Second,
		RecordTTL:       time.Duration(getEnvFloat("RECORD_TTL_SECONDS", 86400)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}