package auth_test

import (
	"context"
	"testing"
	"time"

	"session-service/internal/auth"
	"session-service/test/mocks"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateToken_MissingKidFails(t *testing.T) {
	privPEM, pubPEM := generateTestPEMKeys(t)
	km, err := auth.NewKeyManager(privPEM, pubPEM)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	cacheMock := &mocks.MockCache{}
	validator := auth.NewTokenValidator(km, "issuer", "audience", cacheMock)

	// Build a token without kid header
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": "issuer",
		"aud": "audience",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(km.GetPrivateKey())
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = validator.ValidateToken(context.Background(), signed)
	if err == nil {
		t.Fatalf("expected error due to missing kid, got nil")
	}
}


