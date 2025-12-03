package auth

import (
	"context"
	"fmt"
	"session-service/internal/cache"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenValidator handles token validation
type TokenValidator struct {
	keyManager *KeyManager
	issuer     string
	audience   string
	cache      cache.Cache
}

// NewTokenValidator creates a new token validator
func NewTokenValidator(keyManager *KeyManager, issuer, audience string, cache cache.Cache) *TokenValidator {
	return &TokenValidator{
		keyManager: keyManager,
		issuer:     issuer,
		audience:   audience,
		cache:      cache,
	}
}

// ValidateToken validates a JWT token
func (tv *TokenValidator) ValidateToken(ctx context.Context, tokenString string) (jwt.MapClaims, error) {
	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Require kid so we always pick an explicit key; no fallback.
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("missing kid in token header")
		}
		pub, err := tv.keyManager.GetPublicKeyByID(kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key for kid %s: %w", kid, err)
		}
		return pub, nil
	}, jwt.WithValidMethods([]string{"RS256"}))

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if iss, ok := claims["iss"].(string); !ok || iss != tv.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Validate audience
	if aud, ok := claims["aud"].(string); !ok || aud != tv.audience {
		return nil, fmt.Errorf("invalid audience")
	}

	// Check expiration (jwt-go already validates this, but double-check)
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("token has expired")
		}
	}

	// Check revocation list
	if jti, ok := claims["jti"].(string); ok && jti != "" {
		revoked, err := tv.cache.IsTokenRevoked(ctx, jti)
		if err != nil {
			return nil, fmt.Errorf("failed to check token revocation: %w", err)
		}
		if revoked {
			return nil, fmt.Errorf("token has been revoked")
		}
	}

	return claims, nil
}
