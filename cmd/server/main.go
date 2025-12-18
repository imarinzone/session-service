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
	"syscall"
	"time"

	"go.uber.org/zap"

	_ "session-service/docs" // swagger docs
)

// @title           Session Service API
// @version         1.0
// @description     OAuth2/OIDC session service with multi-tenant support and JWT token management
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:9090
// @BasePath  /

// @securityDefinitions.basic  BasicAuth
// @securityDefinitions.apikey  BearerAuth
// @in                         header
// @name                       Authorization
// @description                Bearer token authentication. Format: "Bearer {token}"

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
	jwksHandler := handlers.NewJWKSHandler(repo, keyManager, logger)
	oidcHandler := handlers.NewOIDCConfigurationHandler(cfg.BaseURL, cfg.JWTIssuer, logger)

	// Setup router
	router := SetupRouter(tokenHandler, verifyHandler, jwksHandler, oidcHandler, logger)

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
