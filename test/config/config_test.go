package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"session-service/internal/config"
)

// Helper to generate PEM keys
func generateTestPEMKeys(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test keys: %v", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(privPEM), string(pubPEM)
}

func TestLoad(t *testing.T) {
	// Generate valid keys
	privKey, pubKey := generateTestPEMKeys(t)

	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			env: map[string]string{
				"JWT_PRIVATE_KEY": privKey,
				"JWT_PUBLIC_KEY":  pubKey,
				"DATABASE_URL":    "postgres://user:pass@localhost:5432/db",
				"REDIS_URL":       "redis://localhost:6379/0",
			},
			wantErr: false,
		},
		{
			name: "missing keys",
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
			},
			wantErr: true,
		},
		{
			name: "invalid private key",
			env: map[string]string{
				"JWT_PRIVATE_KEY": "invalid-key",
				"JWT_PUBLIC_KEY":  pubKey,
			},
			wantErr: true,
		},
		{
			name: "placeholder keys",
			env: map[string]string{
				"JWT_PRIVATE_KEY": "...",
				"JWT_PUBLIC_KEY":  "...",
			},
			wantErr: true,
		},
		{
			name: "custom duration",
			env: map[string]string{
				"JWT_PRIVATE_KEY": privKey,
				"JWT_PUBLIC_KEY":  pubKey,
				"JWT_EXPIRY":      "2h",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env before each test
			os.Clearenv()

			// Set env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := config.Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg == nil {
				t.Error("Load() returned nil config without error")
			}

			// Verify custom duration if applicable
			if !tt.wantErr && tt.env["JWT_EXPIRY"] == "2h" {
				if cfg.JWTExpiry != 2*time.Hour {
					t.Errorf("JWTExpiry = %v, want %v", cfg.JWTExpiry, 2*time.Hour)
				}
			}
		})
	}
}
