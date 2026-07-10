package main

import (
	"log"
	"net/http"
	"time"
	"webhook-dispatcher/internal/config"
	"webhook-dispatcher/internal/handlers"
)

func main() {
	cfg := config.Load()

	if cfg.APIKey == "" {
		log.Println("WARNING: API_KEY is not set, all endpoints are open.")
	}
	log.Printf("Starting webhook dispatcher on :%s (max_attempts=%d, backoff=%s base)", cfg.Port, cfg.MaxAttempts, cfg.BackoffBase)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Must be > max total delivery time
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}