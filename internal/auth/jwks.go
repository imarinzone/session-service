package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// KeyManager manages JWT keys and JWKS
type KeyManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keySet     jwk.Set
}

// NewKeyManager creates a new key manager from PEM-encoded keys
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

	// Create JWK set
	keySet, err := createJWKSet(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWK set: %w", err)
	}

	return &KeyManager{
		privateKey: privateKey,
		publicKey:  publicKey,
		keySet:     keySet,
	}, nil
}

// GetPrivateKey returns the private key
func (km *KeyManager) GetPrivateKey() *rsa.PrivateKey {
	return km.privateKey
}

// GetPublicKey returns the public key
func (km *KeyManager) GetPublicKey() *rsa.PublicKey {
	return km.publicKey
}

// GetJWKSet returns the JWK set for JWKS endpoint
func (km *KeyManager) GetJWKSet() jwk.Set {
	return km.keySet
}

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

func createJWKSet(publicKey *rsa.PublicKey) (jwk.Set, error) {
	key, err := jwk.FromRaw(publicKey)
	if err != nil {
		return nil, err
	}

	if err := key.Set(jwk.KeyIDKey, "session-service-key"); err != nil {
		return nil, err
	}
	if err := key.Set(jwk.AlgorithmKey, "RS256"); err != nil {
		return nil, err
	}

	keySet := jwk.NewSet()
	if err := keySet.AddKey(key); err != nil {
		return nil, err
	}

	return keySet, nil
}

