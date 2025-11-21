package middleware

import (
	"net/http"
	"session-service/internal/cache"
	"session-service/pkg/errors"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(cache *cache.Cache, logger *zap.Logger, defaultLimit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client_id from context (set by token handler)
			clientID := r.Context().Value("client_id")
			if clientID == nil {
				// If no client_id, skip rate limiting (shouldn't happen in normal flow)
				next.ServeHTTP(w, r)
				return
			}

			clientIDStr := clientID.(string)
			limit := defaultLimit

			// Get client-specific limit from context if available
			if clientLimit := r.Context().Value("client_rate_limit"); clientLimit != nil {
				limit = clientLimit.(int)
			}

			ctx := r.Context()
			exceeded, err := cache.CheckRateLimit(ctx, clientIDStr, limit, window)
			if err != nil {
				logger.Error("Rate limit check failed", zap.Error(err))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if exceeded {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				w.WriteHeader(errors.ErrRateLimitExceeded.Status)
				w.Write([]byte(`{"error":"` + errors.ErrRateLimitExceeded.Code + `","error_description":"` + errors.ErrRateLimitExceeded.Message + `"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

