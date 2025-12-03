package models

import "time"

// Tenant represents a tenant in the database
type Tenant struct {
	ID          string    `db:"id"`
	ExternalTID string    `db:"external_tid"`
	Name        string    `db:"name"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// User represents a user in the database (opaque IDs, no PII in tokens)
type User struct {
	ID          string    `db:"id"`
	TenantID    string    `db:"tenant_id"`
	Email       string    `db:"email"`        // PII, never put in tokens
	FullName    string    `db:"full_name"`   // PII, never put in tokens
	PhoneNumber string    `db:"phone_number"`// PII, never put in tokens
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// UserRole represents a role assignment for a user within a tenant
type UserRole struct {
	UserID string `db:"user_id"`
	Role   string `db:"role"`
}

// Client represents a client in the database
type Client struct {
	ID               int64     `db:"id"`
	ClientID         string    `db:"client_id"`
	ClientSecretHash string    `db:"client_secret_hash"`
	RateLimit        int       `db:"rate_limit"`
	TenantID         string    `db:"tenant_id"`
	UserID           string    `db:"user_id"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
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
// It carries the original client and subject so refresh tokens can issue
// the same user/tenant-scoped access tokens without re-reading from DB.
type RefreshTokenData struct {
	ClientID string        `json:"client_id"`
	Subject  *TokenSubject `json:"subject,omitempty"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// TokenSubject represents the identity and authorization context for a token
// It is used to construct minimal, non-PII JWT claims (sub, tid, roles, scp, etc.).
type TokenSubject struct {
	UserID   string   // maps to sub / oid
	TenantID string   // maps to tid
	Roles    []string // roles claim
	Scopes   []string // scp claim
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

