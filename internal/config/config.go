package config

import (
	"os"
	"strconv"
	"time"
)

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
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/sessiondb?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTPrivateKey:      getEnv("JWT_PRIVATE_KEY", ""),
		JWTPublicKey:       getEnv("JWT_PUBLIC_KEY", ""),
		JWTIssuer:          getEnv("JWT_ISSUER", "session-service"),
		JWTAudience:        getEnv("JWT_AUDIENCE", "api"),
		JWTExpiry:          getDurationEnv("JWT_EXPIRY", 3600*time.Second),
		RefreshTokenExpiry: getDurationEnv("REFRESH_TOKEN_EXPIRY", 7*24*3600*time.Second),
		RefreshTokenLength: getIntEnv("REFRESH_TOKEN_LENGTH", 32),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
	}

	if cfg.JWTPrivateKey == "" || cfg.JWTPublicKey == "" {
		return nil, &ConfigError{Message: "JWT_PRIVATE_KEY and JWT_PUBLIC_KEY must be set"}
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

