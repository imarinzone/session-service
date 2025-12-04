package main

import (
	"net/http"
	"session-service/internal/handlers"
	"session-service/internal/middleware"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// SetupRouter configures and returns the HTTP router with all routes and middleware
func SetupRouter(
	tokenHandler *handlers.TokenHandler,
	verifyHandler *handlers.VerifyHandler,
	jwksHandler *handlers.JWKSHandler,
	oidcHandler *handlers.OIDCConfigurationHandler,
	logger *zap.Logger,
) *mux.Router {
	router := mux.NewRouter()

	// Add CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Add logging middleware
	router.Use(middleware.LoggingMiddleware(logger))

	// OIDC Discovery (not tenant-scoped)
	router.HandleFunc("/.well-known/openid-configuration", oidcHandler.HandleOIDCConfiguration).Methods("GET", "OPTIONS")

	// OAuth2 endpoints (tenant-scoped)
	router.HandleFunc("/{tenant_id}/oauth2/v2.0/token", tokenHandler.HandleToken).Methods("POST", "OPTIONS")
	router.HandleFunc("/{tenant_id}/discovery/v1.0/keys", jwksHandler.HandleJWKS).Methods("GET", "OPTIONS")

	// Verify Token (tenant-scoped)
	router.HandleFunc("/{tenant_id}/oauth2/v1.0/verify", verifyHandler.HandleVerify).Methods("POST", "OPTIONS")

	// Health check (tenant-scoped)
	// @Summary     Health check endpoint
	// @Description Returns OK if the service is running
	// @Tags        health
	// @Produce     text/plain
	// @Success     200  {string}  string  "OK"
	// @Router      /{tenant_id}/health [get]
	router.HandleFunc("/{tenant_id}/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Swagger documentation
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	return router
}
