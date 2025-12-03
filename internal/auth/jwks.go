package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// KeyPair represents a single signing key and its metadata.
type KeyPair struct {
	KeyID      string
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	CreatedAt  time.Time
	ExpiresAt  time.Time
	IsActive   bool
}

// KeyManager manages JWT keys, rotation, and JWKS.
// It is designed to support multiple active keys (current + previous) like Azure AD / Hydra.
type KeyManager struct {
	mu           sync.RWMutex
	keys         map[string]*KeyPair
	currentKeyID string
}

// NewKeyManager creates a new key manager from an initial PEM-encoded key pair.
// Additional keys may be generated at runtime for rotation.
func NewKeyManager(privateKeyPEM, publicKeyPEM string) (*KeyManager, error) {
	// Parse private key
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Parse public key
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	keyID := uuid.New().String()
	now := time.Now()

	initialKey := &KeyPair{
		KeyID:      keyID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  now,
		// ExpiresAt is managed by rotation logic; zero means no explicit expiry yet.
		IsActive: true,
	}

	return &KeyManager{
		keys: map[string]*KeyPair{
			keyID: initialKey,
		},
		currentKeyID: keyID,
	}, nil
}

// GetPrivateKey returns the current private key used for signing.
func (km *KeyManager) GetPrivateKey() *rsa.PrivateKey {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if key, ok := km.keys[km.currentKeyID]; ok && key.IsActive {
		return key.PrivateKey
	}
	return nil
}

// GetCurrentKeyID returns the kid of the current signing key.
func (km *KeyManager) GetCurrentKeyID() string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentKeyID
}

// GetPublicKeyByID returns the public key for a given kid, if present and active.
func (km *KeyManager) GetPublicKeyByID(keyID string) (*rsa.PublicKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, ok := km.keys[keyID]
	if !ok || !key.IsActive {
		return nil, fmt.Errorf("key not found or inactive: %s", keyID)
	}
	if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("key expired: %s", keyID)
	}
	return key.PublicKey, nil
}

// GetJWKSet returns the JWK set for JWKS endpoint containing all active keys.
func (km *KeyManager) GetJWKSet() jwk.Set {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keySet := jwk.NewSet()
	now := time.Now()

	for _, kp := range km.keys {
		if !kp.IsActive {
			continue
		}
		if !kp.ExpiresAt.IsZero() && kp.ExpiresAt.Before(now) {
			continue
		}

		jwkKey, err := jwk.FromRaw(kp.PublicKey)
		if err != nil {
			continue
		}
		_ = jwkKey.Set(jwk.KeyIDKey, kp.KeyID)
		_ = jwkKey.Set(jwk.AlgorithmKey, "RS256")
		_ = jwkKey.Set(jwk.KeyUsageKey, "sig")

		_ = keySet.AddKey(jwkKey)
	}

	return keySet
}

// RotateKeys generates a new key pair and marks the old one for graceful deactivation.
// gracePeriod defines how long the old key remains valid for verification.
func (km *KeyManager) RotateKeys(gracePeriod time.Duration) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Generate new key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate new RSA key: %w", err)
	}
	publicKey := &privateKey.PublicKey

	keyID := uuid.New().String()
	now := time.Now()

	newKey := &KeyPair{
		KeyID:      keyID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  now,
		IsActive:   true,
	}

	// Mark previous current key to expire after gracePeriod
	if current, ok := km.keys[km.currentKeyID]; ok {
		current.ExpiresAt = now.Add(gracePeriod)
	}

	km.keys[keyID] = newKey
	km.currentKeyID = keyID

	return nil
}

// CleanupExpiredKeys removes keys that are past their ExpiresAt.
func (km *KeyManager) CleanupExpiredKeys() {
	km.mu.Lock()
	defer km.mu.Unlock()

	now := time.Now()
	for id, kp := range km.keys {
		if !kp.ExpiresAt.IsZero() && kp.ExpiresAt.Before(now) {
			delete(km.keys, id)
		}
	}
}

// parseRSAPrivateKey parses a PEM-encoded RSA private key.
func parseRSAPrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("key is not an RSA private key")
		}
		return rsaKey, nil
	}

	return key, nil
}

// parseRSAPublicKey parses a PEM-encoded RSA public key.
func parseRSAPublicKey(pemData string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	return rsaKey, nil
}
