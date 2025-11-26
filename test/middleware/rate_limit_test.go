package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"session-service/internal/middleware"
	"session-service/test/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Setup
	mockCache := new(mocks.MockCache)
	logger := zap.NewNop()

	mw := middleware.RateLimitMiddleware(mockCache, logger, 10, time.Minute)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(testHandler)

	t.Run("Allowed", func(t *testing.T) {
		// Mock expectation
		mockCache.On("CheckRateLimit", mock.Anything, "client-1", 10, time.Minute).Return(false, nil).Once()

		req := httptest.NewRequest("GET", "/", nil)
		// Inject client_id into context
		ctx := context.WithValue(req.Context(), "client_id", "client-1")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Exceeded", func(t *testing.T) {
		// Mock expectation
		mockCache.On("CheckRateLimit", mock.Anything, "client-2", 10, time.Minute).Return(true, nil).Once()

		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), "client_id", "client-2")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	})

	t.Run("NoClientID", func(t *testing.T) {
		// Should skip rate limiting if no client_id
		req := httptest.NewRequest("GET", "/", nil)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
