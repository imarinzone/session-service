package models

import "time"

// Client represents a client in the database
type Client struct {
	ID             int64     `db:"id"`
	ClientID       string    `db:"client_id"`
	ClientSecretHash string  `db:"client_secret_hash"`
	RateLimit      int       `db:"rate_limit"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// TokenResponse represents the OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// TokenRequest represents the OAuth2 token request
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenData represents refresh token data stored in Redis
type RefreshTokenData struct {
	ClientID  string    `json:"client_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// VerifyRequest represents a token verification request
type VerifyRequest struct {
	Token string `json:"token"`
}

// VerifyResponse represents a token verification response
type VerifyResponse struct {
	Valid   bool                   `json:"valid"`
	Claims  map[string]interface{} `json:"claims,omitempty"`
	Message string                 `json:"message,omitempty"`
}

