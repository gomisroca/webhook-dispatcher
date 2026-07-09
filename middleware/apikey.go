package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

func RequireAPIKey(apiKey string, next http.Handler) http.Handler {
	if apiKey == "" {
		return next 
	}

	expected := []byte(apiKey)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get("X-API-Key"))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid API key"})
			return
		}
		next.ServeHTTP(w, r)
	})
}