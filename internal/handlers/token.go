package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"session-service/internal/auth"
	"session-service/internal/cache"
	"session-service/internal/config"
	"session-service/internal/database"
	"session-service/internal/models"
	"session-service/pkg/errors"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// TokenHandler handles OAuth2 token requests
type TokenHandler struct {
	repo           *database.Repository
	cache          *cache.Cache
	tokenGen       *auth.TokenGenerator
	tokenValidator *auth.TokenValidator
	config         *config.Config
	logger         *zap.Logger
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(
	repo *database.Repository,
	cache *cache.Cache,
	tokenGen *auth.TokenGenerator,
	tokenValidator *auth.TokenValidator,
	config *config.Config,
	logger *zap.Logger,
) *TokenHandler {
	return &TokenHandler{
		repo:           repo,
		cache:          cache,
		tokenGen:       tokenGen,
		tokenValidator: tokenValidator,
		config:         config,
		logger:         logger,
	}
}

// HandleToken handles POST /oauth/token
func (h *TokenHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "client_credentials":
		h.handleClientCredentials(ctx, w, r)
	case "refresh_token":
		h.handleRefreshToken(ctx, w, r)
	default:
		h.sendError(w, errors.ErrInvalidGrant)
	}
}

func (h *TokenHandler) handleClientCredentials(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	if clientID == "" || clientSecret == "" {
		h.sendError(w, errors.ErrInvalidCredentials)
		return
	}

	// Check cache first
	client, err := h.cache.GetClient(ctx, clientID)
	if err != nil {
		h.logger.Error("Failed to get client from cache", zap.Error(err))
	}

	// If not in cache, get from database
	if client == nil {
		client, err = h.repo.GetClientByID(ctx, clientID)
		if err != nil {
			h.logger.Error("Failed to get client from database", zap.Error(err))
			h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
			return
		}

		if client == nil {
			h.sendError(w, errors.ErrInvalidCredentials)
			return
		}

		// Cache the client
		if err := h.cache.SetClient(ctx, client, 15*time.Minute); err != nil {
			h.logger.Warn("Failed to cache client", zap.Error(err))
		}
	}

	// Verify client secret
	if err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(clientSecret)); err != nil {
		h.sendError(w, errors.ErrInvalidCredentials)
		return
	}

	// Check rate limit
	exceeded, err := h.cache.CheckRateLimit(ctx, clientID, client.RateLimit, time.Minute)
	if err != nil {
		h.logger.Error("Rate limit check failed", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}
	if exceeded {
		h.sendError(w, errors.ErrRateLimitExceeded)
		return
	}

	// Generate tokens
	accessToken, _, err := h.tokenGen.GenerateAccessToken(clientID)
	if err != nil {
		h.logger.Error("Failed to generate access token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	refreshToken, err := h.tokenGen.GenerateRefreshToken()
	if err != nil {
		h.logger.Error("Failed to generate refresh token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	// Store refresh token
	refreshTokenData := &models.RefreshTokenData{
		ClientID:  clientID,
		ExpiresAt: time.Now().Add(h.config.RefreshTokenExpiry),
	}
	if err := h.cache.StoreRefreshToken(ctx, refreshToken, refreshTokenData, h.config.RefreshTokenExpiry); err != nil {
		h.logger.Error("Failed to store refresh token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	// Update client updated_at
	if err := h.repo.UpdateClientUpdatedAt(ctx, clientID); err != nil {
		h.logger.Warn("Failed to update client updated_at", zap.Error(err))
	}

	// Send response
	response := &models.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.config.JWTExpiry.Seconds()),
		RefreshToken: refreshToken,
	}

	h.sendJSON(w, http.StatusOK, response)
}

func (h *TokenHandler) handleRefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")

	if refreshToken == "" {
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	// Get refresh token data from cache
	tokenData, err := h.cache.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		h.logger.Error("Failed to get refresh token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	if tokenData == nil {
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	// Check if refresh token is revoked
	revoked, err := h.cache.IsRefreshTokenRevoked(ctx, refreshToken)
	if err != nil {
		h.logger.Error("Failed to check refresh token revocation", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}
	if revoked {
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	// Check if refresh token is expired
	if time.Now().After(tokenData.ExpiresAt) {
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	clientID := tokenData.ClientID

	// Get client to check rate limit
	client, err := h.repo.GetClientByID(ctx, clientID)
	if err != nil {
		h.logger.Error("Failed to get client from database", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	if client == nil {
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	// Check rate limit
	exceeded, err := h.cache.CheckRateLimit(ctx, clientID, client.RateLimit, time.Minute)
	if err != nil {
		h.logger.Error("Rate limit check failed", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}
	if exceeded {
		h.sendError(w, errors.ErrRateLimitExceeded)
		return
	}

	// Revoke old refresh token
	if err := h.cache.RevokeRefreshToken(ctx, refreshToken, h.config.RefreshTokenExpiry); err != nil {
		h.logger.Warn("Failed to revoke old refresh token", zap.Error(err))
	}
	if err := h.cache.DeleteRefreshToken(ctx, refreshToken); err != nil {
		h.logger.Warn("Failed to delete old refresh token", zap.Error(err))
	}

	// Generate new tokens
	accessToken, _, err := h.tokenGen.GenerateAccessToken(clientID)
	if err != nil {
		h.logger.Error("Failed to generate access token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	newRefreshToken, err := h.tokenGen.GenerateRefreshToken()
	if err != nil {
		h.logger.Error("Failed to generate refresh token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	// Store new refresh token
	newRefreshTokenData := &models.RefreshTokenData{
		ClientID:  clientID,
		ExpiresAt: time.Now().Add(h.config.RefreshTokenExpiry),
	}
	if err := h.cache.StoreRefreshToken(ctx, newRefreshToken, newRefreshTokenData, h.config.RefreshTokenExpiry); err != nil {
		h.logger.Error("Failed to store refresh token", zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	// Send response
	response := &models.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.config.JWTExpiry.Seconds()),
		RefreshToken: newRefreshToken,
	}

	h.sendJSON(w, http.StatusOK, response)
}

func (h *TokenHandler) sendError(w http.ResponseWriter, err *errors.ServiceError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             err.Code,
		"error_description": err.Message,
	})
}

func (h *TokenHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
