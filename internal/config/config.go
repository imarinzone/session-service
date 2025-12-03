package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Config holds all configuration for the application
type Config struct {
	DatabaseURL        string
	RedisURL           string
	JWTPrivateKey      string
	JWTPublicKey       string
	JWTIssuer          string
	JWTAudience        string
	JWTExpiry          time.Duration
	RefreshTokenExpiry time.Duration
	RefreshTokenLength int
	ServerPort         string
	BaseURL            string
	KeyRotationDays    int
	KeyGraceDays       int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	var jwtPrivateKey, jwtPublicKey string

	// Fallback to plain keys with escaped newlines
	jwtPrivateKey = getEnv("JWT_PRIVATE_KEY", "")
	jwtPublicKey = getEnv("JWT_PUBLIC_KEY", "")

	// If keys are empty, provide helpful error
	if jwtPrivateKey == "" && jwtPublicKey == "" {
		return nil, &ConfigError{Message: "JWT_PRIVATE_KEY and JWT_PUBLIC_KEY must be set. Generate keys using: make generate-keys"}
	}

	cfg := &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/sessiondb?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTPrivateKey:      jwtPrivateKey,
		JWTPublicKey:       jwtPublicKey,
		JWTIssuer:          getEnv("JWT_ISSUER", "session-service"),
		JWTAudience:        getEnv("JWT_AUDIENCE", "api"),
		JWTExpiry:          getDurationEnv("JWT_EXPIRY", 3600*time.Second),
		RefreshTokenExpiry: getDurationEnv("REFRESH_TOKEN_EXPIRY", 7*24*3600*time.Second),
		RefreshTokenLength: getIntEnv("REFRESH_TOKEN_LENGTH", 32),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		BaseURL:            getEnv("BASE_URL", "http://localhost:8080"),
		KeyRotationDays:    getIntEnv("KEY_ROTATION_DAYS", 90),
		KeyGraceDays:       getIntEnv("KEY_GRACE_DAYS", 14),
	}

	if cfg.JWTPrivateKey == "" || cfg.JWTPublicKey == "" {
		return nil, &ConfigError{Message: "JWT_PRIVATE_KEY and JWT_PUBLIC_KEY must be set. Generate keys using: make generate-keys"}
	}

	// Validate that keys contain PEM markers
	if !strings.Contains(cfg.JWTPrivateKey, "BEGIN") || !strings.Contains(cfg.JWTPrivateKey, "END") {
		return nil, &ConfigError{Message: fmt.Sprintf("JWT_PRIVATE_KEY does not appear to be a valid PEM key. Got length: %d. First 50 chars: %s. Check that keys are properly set in .env file. Run: make generate-keys", len(cfg.JWTPrivateKey), cfg.JWTPrivateKey[:min(50, len(cfg.JWTPrivateKey))])}
	}
	if !strings.Contains(cfg.JWTPublicKey, "BEGIN") || !strings.Contains(cfg.JWTPublicKey, "END") {
		return nil, &ConfigError{Message: fmt.Sprintf("JWT_PUBLIC_KEY does not appear to be a valid PEM key. Got length: %d. First 50 chars: %s. Check that keys are properly set in .env file. Run: make generate-keys", len(cfg.JWTPublicKey), cfg.JWTPublicKey[:min(50, len(cfg.JWTPublicKey))])}
	}

	// Check if keys are placeholder values
	if strings.Contains(cfg.JWTPrivateKey, "...") || strings.Contains(cfg.JWTPublicKey, "...") {
		return nil, &ConfigError{Message: "JWT keys appear to be placeholder values. Please generate real keys using: make generate-keys"}
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		// Try parsing as seconds
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
