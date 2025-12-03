package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"session-service/internal/auth"
	"session-service/internal/cache"
	"session-service/internal/config"
	"session-service/internal/database"
	"session-service/internal/handlers"
	"session-service/internal/middleware"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("Starting session service")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database
	ctx := context.Background()
	repo, err := database.NewRepository(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer repo.Close()

	// Initialize cache
	cacheClient, err := cache.NewCache(cfg.RedisURL, logger)
	if err != nil {
		logger.Fatal("Failed to initialize cache", zap.Error(err))
	}
	defer cacheClient.Close()

	// Initialize key manager
	keyManager, err := auth.NewKeyManager(cfg.JWTPrivateKey, cfg.JWTPublicKey)
	if err != nil {
		logger.Fatal("Failed to initialize key manager", zap.Error(err))
	}

	// Start key rotation scheduler (Azure/Hydra-style)
	go func() {
		rotationDays := cfg.KeyRotationDays
		if rotationDays <= 0 {
			rotationDays = 90
		}
		graceDays := cfg.KeyGraceDays
		if graceDays <= 0 {
			graceDays = 14
		}

		rotationInterval := time.Duration(rotationDays) * 24 * time.Hour
		gracePeriod := time.Duration(graceDays) * 24 * time.Hour

		ticker := time.NewTicker(rotationInterval)
		defer ticker.Stop()

		for range ticker.C {
			logger.Info("Rotating signing keys", zap.Int("rotation_days", rotationDays), zap.Int("grace_days", graceDays))
			if err := keyManager.RotateKeys(gracePeriod); err != nil {
				logger.Error("Failed to rotate keys", zap.Error(err))
			}
			keyManager.CleanupExpiredKeys()
		}
	}()

	// Initialize token generator
	tokenGen := auth.NewTokenGenerator(
		keyManager,
		cfg.JWTIssuer,
		cfg.JWTAudience,
		cfg.JWTExpiry,
		cfg.RefreshTokenLength,
	)

	// Initialize token validator
	tokenValidator := auth.NewTokenValidator(
		keyManager,
		cfg.JWTIssuer,
		cfg.JWTAudience,
		cacheClient,
	)

	// Initialize handlers
	tokenHandler := handlers.NewTokenHandler(
		repo,
		cacheClient,
		tokenGen,
		tokenValidator,
		cfg,
		logger,
	)

	verifyHandler := handlers.NewVerifyHandler(tokenValidator, logger)
	jwksHandler := handlers.NewJWKSHandler(keyManager, logger)
	oidcHandler := handlers.NewOIDCConfigurationHandler(cfg.BaseURL, cfg.JWTIssuer, logger)

	// Setup router
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

	// Routes
	// OIDC Discovery
	router.HandleFunc("/.well-known/openid-configuration", oidcHandler.HandleOIDCConfiguration).Methods("GET", "OPTIONS")

	// OAuth2 endpoints
	router.HandleFunc("/oauth2/v1.0/token", tokenHandler.HandleToken).Methods("POST", "OPTIONS")
	router.HandleFunc("/discovery/v1.0/keys", jwksHandler.HandleJWKS).Methods("GET", "OPTIONS")

	// Verfy Token
	router.HandleFunc("/oauth2/v1.0/verify", verifyHandler.HandleVerify).Methods("POST", "OPTIONS")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server starting", zap.String("port", cfg.ServerPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
