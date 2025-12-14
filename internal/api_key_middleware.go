package internal

import (
	"context"
	"net/http"
	"strings"
)

// ========================
// API Key Middleware
// ========================
func RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			http.Error(w, "missing or invalid API key", http.StatusUnauthorized)
			return
		}

		key := strings.TrimPrefix(auth, prefix)

		if _, ok := APIKeys[key]; !ok {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		// store key in context for rate limiter
		ctx := context.WithValue(r.Context(), "api_key", key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
