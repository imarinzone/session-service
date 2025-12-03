package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"session-service/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenGenerator handles token generation
type TokenGenerator struct {
	keyManager         *KeyManager
	issuer             string
	audience           string
	accessTokenExpiry  time.Duration
	refreshTokenLength int
}

// NewTokenGenerator creates a new token generator
func NewTokenGenerator(keyManager *KeyManager, issuer, audience string, accessTokenExpiry time.Duration, refreshTokenLength int) *TokenGenerator {
	return &TokenGenerator{
		keyManager:         keyManager,
		issuer:             issuer,
		audience:           audience,
		accessTokenExpiry:  accessTokenExpiry,
		refreshTokenLength: refreshTokenLength,
	}
}

// GenerateAccessToken generates a JWT access token using a TokenSubject.
// All access tokens are user/tenant scoped; there is no client-only fallback.
func (tg *TokenGenerator) GenerateAccessToken(subject *models.TokenSubject) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"iss": tg.issuer,
		"aud": tg.audience,
		"exp": now.Add(tg.accessTokenExpiry).Unix(),
		"iat": now.Unix(),
		"jti": jti,
	}

	// subject is required; we assume caller has validated it.
	claims["sub"] = subject.UserID
	claims["oid"] = subject.UserID
	claims["tid"] = subject.TenantID
	if len(subject.Roles) > 0 {
		claims["roles"] = subject.Roles
	}
	if len(subject.Scopes) > 0 {
		claims["scp"] = subject.Scopes
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Set kid header so verifiers can select the correct key from JWKS when rotation is enabled.
	if tg.keyManager != nil {
		if kid := tg.keyManager.GetCurrentKeyID(); kid != "" {
			token.Header["kid"] = kid
		}
	}

	tokenString, err := token.SignedString(tg.keyManager.GetPrivateKey())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, nil
}

// GenerateRefreshToken generates a random refresh token
func (tg *TokenGenerator) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, tg.refreshTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
