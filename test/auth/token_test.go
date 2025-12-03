package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"session-service/internal/auth"
	"session-service/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// Helper function to generate test RSA keys and return them as PEM strings
func generateTestPEMKeys(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test keys: %v", err)
	}

	// Encode private key to PEM
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	// Encode public key to PEM
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

// Helper function to create a test KeyManager
func createTestKeyManager(t *testing.T) *auth.KeyManager {
	t.Helper()
	privPEM, pubPEM := generateTestPEMKeys(t)

	km, err := auth.NewKeyManager(privPEM, pubPEM)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	return km
}

func TestNewTokenGenerator(t *testing.T) {
	tests := []struct {
		name               string
		issuer             string
		audience           string
		accessTokenExpiry  time.Duration
		refreshTokenLength int
	}{
		{
			name:               "valid configuration",
			issuer:             "https://auth.example.com",
			audience:           "https://api.example.com",
			accessTokenExpiry:  15 * time.Minute,
			refreshTokenLength: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := createTestKeyManager(t)

			tg := auth.NewTokenGenerator(km, tt.issuer, tt.audience, tt.accessTokenExpiry, tt.refreshTokenLength)

			if tg == nil {
				t.Fatal("NewTokenGenerator returned nil")
			}
		})
	}
}

func TestGenerateAccessToken(t *testing.T) {
	km := createTestKeyManager(t)
	issuer := "https://auth.example.com"
	audience := "https://api.example.com"
	accessTokenExpiry := 15 * time.Minute

	tg := auth.NewTokenGenerator(km, issuer, audience, accessTokenExpiry, 32)

	tests := []struct {
		name    string
		subject *models.TokenSubject
	}{
		{
			name: "basic subject",
			subject: &models.TokenSubject{
				UserID:   "user-123",
				TenantID: "tenant-abc",
				Roles:    []string{"reader"},
				Scopes:   []string{"sessions:read"},
			},
		},
		{
			name: "subject without roles/scopes",
			subject: &models.TokenSubject{
				UserID:   "user-456",
				TenantID: "tenant-def",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, jti, err := tg.GenerateAccessToken(tt.subject)

			if err != nil {
				t.Fatalf("GenerateAccessToken() error = %v", err)
			}

			if tokenString == "" {
				t.Error("tokenString is empty")
			}

			if jti == "" {
				t.Error("jti is empty")
			}

			// Verify the token can be parsed
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Verify signing method
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					t.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				// Use the current public key from the key manager
				return km.GetPrivateKey().Public(), nil
			})

			if err != nil {
				t.Fatalf("failed to parse token: %v", err)
			}

			if !token.Valid {
				t.Error("token is not valid")
			}

			// Verify claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				t.Fatal("failed to get claims")
			}

			// Check issuer
			if iss, ok := claims["iss"].(string); !ok || iss != issuer {
				t.Errorf("issuer = %v, want %v", iss, issuer)
			}

			// Check audience
			if aud, ok := claims["aud"].(string); !ok || aud != audience {
				t.Errorf("audience = %v, want %v", aud, audience)
			}

			// Check subject (userID)
			if sub, ok := claims["sub"].(string); !ok || sub != tt.subject.UserID {
				t.Errorf("subject = %v, want %v", sub, tt.subject.UserID)
			}

			// Check oid
			if oid, ok := claims["oid"].(string); !ok || oid != tt.subject.UserID {
				t.Errorf("oid = %v, want %v", oid, tt.subject.UserID)
			}

			// Check tid
			if tid, ok := claims["tid"].(string); !ok || tid != tt.subject.TenantID {
				t.Errorf("tid = %v, want %v", tid, tt.subject.TenantID)
			}

			// Check roles, if expected
			if len(tt.subject.Roles) > 0 {
				if roles, ok := claims["roles"].([]interface{}); ok {
					if len(roles) != len(tt.subject.Roles) {
						t.Errorf("roles length = %d, want %d", len(roles), len(tt.subject.Roles))
					}
				} else {
					t.Errorf("roles claim missing or wrong type")
				}
			}

			// Check scopes, if expected
			if len(tt.subject.Scopes) > 0 {
				if scp, ok := claims["scp"].([]interface{}); ok {
					if len(scp) != len(tt.subject.Scopes) {
						t.Errorf("scp length = %d, want %d", len(scp), len(tt.subject.Scopes))
					}
				} else {
					t.Errorf("scp claim missing or wrong type")
				}
			}

			// Check jti
			if claimJti, ok := claims["jti"].(string); !ok || claimJti != jti {
				t.Errorf("jti = %v, want %v", claimJti, jti)
			}

			// Check expiration time
			if exp, ok := claims["exp"].(float64); ok {
				expTime := time.Unix(int64(exp), 0)
				expectedExp := time.Now().Add(accessTokenExpiry)

				// Allow 5 second tolerance for test execution time
				if expTime.Before(expectedExp.Add(-5*time.Second)) || expTime.After(expectedExp.Add(5*time.Second)) {
					t.Errorf("expiration time %v is not within expected range around %v", expTime, expectedExp)
				}
			} else {
				t.Error("exp claim not found or invalid")
			}

			// Check issued at time
			if iat, ok := claims["iat"].(float64); ok {
				iatTime := time.Unix(int64(iat), 0)
				now := time.Now()

				// Allow 5 second tolerance
				if iatTime.Before(now.Add(-5*time.Second)) || iatTime.After(now.Add(5*time.Second)) {
					t.Errorf("issued at time %v is not within expected range around %v", iatTime, now)
				}
			} else {
				t.Error("iat claim not found or invalid")
			}
		})
	}
}

func TestGenerateAccessToken_MultipleCallsProduceDifferentTokens(t *testing.T) {
	km := createTestKeyManager(t)
	tg := auth.NewTokenGenerator(km, "issuer", "audience", 15*time.Minute, 32)

	subject := &models.TokenSubject{
		UserID:   "user-xyz",
		TenantID: "tenant-xyz",
	}

	token1, jti1, err := tg.GenerateAccessToken(subject)
	if err != nil {
		t.Fatalf("first GenerateAccessToken() error = %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	token2, jti2, err := tg.GenerateAccessToken(subject)
	if err != nil {
		t.Fatalf("second GenerateAccessToken() error = %v", err)
	}

	if token1 == token2 {
		t.Error("expected different tokens, got identical tokens")
	}

	if jti1 == jti2 {
		t.Error("expected different JTIs, got identical JTIs")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	km := createTestKeyManager(t)

	tests := []struct {
		name               string
		refreshTokenLength int
		wantMinLength      int // Base64 encoded length will be larger
	}{
		{
			name:               "32 bytes",
			refreshTokenLength: 32,
			wantMinLength:      40, // Base64 encoding increases size
		},
		{
			name:               "16 bytes",
			refreshTokenLength: 16,
			wantMinLength:      20,
		},
		{
			name:               "64 bytes",
			refreshTokenLength: 64,
			wantMinLength:      80,
		},
		{
			name:               "1 byte",
			refreshTokenLength: 1,
			wantMinLength:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := auth.NewTokenGenerator(km, "issuer", "audience", 15*time.Minute, tt.refreshTokenLength)

			token, err := tg.GenerateRefreshToken()

			if err != nil {
				t.Fatalf("GenerateRefreshToken() error = %v", err)
			}

			if token == "" {
				t.Error("token is empty")
			}

			if len(token) < tt.wantMinLength {
				t.Errorf("token length = %v, want at least %v", len(token), tt.wantMinLength)
			}

			// Verify it's valid base64 URL encoding
			_, err = base64.URLEncoding.DecodeString(token)
			if err != nil {
				// Try with padding
				token = strings.TrimRight(token, "=")
				padding := (4 - len(token)%4) % 4
				token += strings.Repeat("=", padding)
				_, err = base64.URLEncoding.DecodeString(token)
				if err != nil {
					t.Errorf("token is not valid base64 URL encoding: %v", err)
				}
			}
		})
	}
}

func TestGenerateRefreshToken_MultipleCallsProduceDifferentTokens(t *testing.T) {
	km := createTestKeyManager(t)
	tg := auth.NewTokenGenerator(km, "issuer", "audience", 15*time.Minute, 32)

	tokens := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		token, err := tg.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken() error = %v", err)
		}

		if tokens[token] {
			t.Errorf("duplicate token generated: %v", token)
		}

		tokens[token] = true
	}

	if len(tokens) != iterations {
		t.Errorf("expected %d unique tokens, got %d", iterations, len(tokens))
	}
}

func TestGenerateRefreshToken_ZeroLength(t *testing.T) {
	km := createTestKeyManager(t)
	tg := auth.NewTokenGenerator(km, "issuer", "audience", 15*time.Minute, 0)

	token, err := tg.GenerateRefreshToken()

	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	// Base64 encoding of 0 bytes should produce empty string
	if token != "" {
		t.Errorf("expected empty token for 0 length, got %v", token)
	}
}
