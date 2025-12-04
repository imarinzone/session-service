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
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// TokenHandler handles OAuth2 token requests
type TokenHandler struct {
	repo           database.Repository
	cache          cache.Cache
	tokenGen       *auth.TokenGenerator
	tokenValidator *auth.TokenValidator
	config         *config.Config
	logger         *zap.Logger
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(
	repo database.Repository,
	cache cache.Cache,
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

// HandleToken handles POST /{tenant_id}/oauth2/v2.0/token
// @Summary     Get OAuth2 access and refresh tokens
// @Description Issues access and refresh tokens using client_credentials, provision_user, or refresh_token grant types. Use provision_user for initial login with user details, client_credentials for subsequent authentication of existing users.
// @Tags        oauth2
// @Accept      application/x-www-form-urlencoded
// @Produce     application/json
// @Param       tenant_id      path     string  true  "Tenant ID"
// @Param       grant_type     formData string  true  "Grant type: client_credentials, provision_user, or refresh_token"
// @Param       client_id      formData string  false "Client ID (required for client_credentials and provision_user)"
// @Param       client_secret  formData string  false "Client Secret (required for client_credentials and provision_user)"
// @Param       user_id       formData string  false "User ID (required for client_credentials and provision_user)"
// @Param       user_full_name formData string  false "User full name (required for provision_user)"
// @Param       user_phone     formData string  false "User phone (required for provision_user)"
// @Param       user_email     formData string  false "User email (optional, provision_user only)"
// @Param       user_roles     formData string  false "Comma-separated user roles (optional, provision_user only)"
// @Param       refresh_token  formData string  false "Refresh token (required for refresh_token grant)"
// @Success     200  {object}  models.TokenResponse
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     500  {object}  map[string]string
// @Router      /{tenant_id}/oauth2/v2.0/token [post]
func (h *TokenHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract tenant_id from URL path
	vars := mux.Vars(r)
	tenantIDFromPath := vars["tenant_id"]
	if tenantIDFromPath == "" {
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "client_credentials":
		h.handleClientCredentials(ctx, w, r, tenantIDFromPath)
	case "provision_user":
		h.handleUserProvisioning(ctx, w, r, tenantIDFromPath)
	case "refresh_token":
		h.handleRefreshToken(ctx, w, r, tenantIDFromPath)
	default:
		h.sendError(w, errors.ErrInvalidGrant)
	}
}

func (h *TokenHandler) handleClientCredentials(ctx context.Context, w http.ResponseWriter, r *http.Request, tenantIDFromPath string) {
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

	// Parse user fields
	userID := r.FormValue("user_id")

	// Use tenant_id from path (required)
	tenantID := tenantIDFromPath

	// Require user_id for this flow; no client-only tokens.
	if userID == "" {
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Ensure tenant exists (strict: no auto-create)
	if err := h.repo.EnsureTenantExists(ctx, tenantID); err != nil {
		h.logger.Error("Tenant does not exist for token request", zap.String("tenant_id", tenantID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInvalidRequest))
		return
	}

	// Get user - must exist for client_credentials flow
	existingUser, err := h.repo.GetUserByID(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user from database", zap.String("user_id", userID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	if existingUser == nil {
		h.logger.Error("User does not exist - use provision_user grant type for first-time login",
			zap.String("user_id", userID))
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Verify tenant matches
	if existingUser.TenantID != tenantID {
		h.logger.Error("User belongs to different tenant",
			zap.String("user_id", userID),
			zap.String("user_tenant_id", existingUser.TenantID),
			zap.String("request_tenant_id", tenantID))
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Get roles from database (no updates)
	roles, err := h.repo.GetUserRoles(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user roles", zap.String("user_id", userID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	subject := &models.TokenSubject{
		UserID:   userID,
		TenantID: tenantID,
		Roles:    roles,
	}

	// Generate tokens
	accessToken, _, err := h.tokenGen.GenerateAccessToken(subject)
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

	// Store refresh token, including subject so refresh can recreate claims
	refreshTokenData := &models.RefreshTokenData{
		ClientID:  clientID,
		Subject:   subject,
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

func (h *TokenHandler) handleUserProvisioning(ctx context.Context, w http.ResponseWriter, r *http.Request, tenantIDFromPath string) {
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

	// Parse user fields
	userID := r.FormValue("user_id")
	userFullName := r.FormValue("user_full_name")
	userPhone := r.FormValue("user_phone")
	userEmail := r.FormValue("user_email")
	userRolesRaw := r.FormValue("user_roles")

	// Use tenant_id from path (required)
	tenantID := tenantIDFromPath

	// Require user_id for this flow
	if userID == "" {
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Require user details for provision flow
	if userFullName == "" || userPhone == "" {
		h.logger.Error("Provision flow requires user_full_name and user_phone",
			zap.String("user_id", userID),
			zap.Bool("has_full_name", userFullName != ""),
			zap.Bool("has_phone", userPhone != ""))
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Ensure tenant exists
	if err := h.repo.EnsureTenantExists(ctx, tenantID); err != nil {
		h.logger.Error("Tenant does not exist for token request", zap.String("tenant_id", tenantID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInvalidRequest))
		return
	}

	// Parse roles if provided
	var roles []string
	if userRolesRaw != "" {
		for _, rStr := range strings.Split(userRolesRaw, ",") {
			rStr = strings.TrimSpace(rStr)
			if rStr != "" {
				roles = append(roles, rStr)
			}
		}
	}

	// Upsert user and roles (this will INSERT or UPDATE)
	user := models.User{
		ID:          userID,
		TenantID:    tenantID,
		Email:       userEmail,
		FullName:    userFullName,
		PhoneNumber: userPhone,
	}

	if err := h.repo.UpsertUserAndRoles(ctx, user, roles); err != nil {
		h.logger.Error("Failed to upsert user and roles", zap.String("user_id", userID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
		return
	}

	// Get roles (either from provided roles or fetch from DB if roles were nil)
	if roles == nil {
		roles, err = h.repo.GetUserRoles(ctx, userID)
		if err != nil {
			h.logger.Error("Failed to get user roles", zap.String("user_id", userID), zap.Error(err))
			h.sendError(w, errors.Wrap(err, errors.ErrInternalServer))
			return
		}
	}

	subject := &models.TokenSubject{
		UserID:   userID,
		TenantID: tenantID,
		Roles:    roles,
	}

	// Generate tokens
	accessToken, _, err := h.tokenGen.GenerateAccessToken(subject)
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

	// Store refresh token, including subject so refresh can recreate claims
	refreshTokenData := &models.RefreshTokenData{
		ClientID:  clientID,
		Subject:   subject,
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

func (h *TokenHandler) handleRefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request, tenantIDFromPath string) {
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
	subject := tokenData.Subject

	// Validate that tenant_id from path matches the tenant_id in the refresh token subject
	if subject == nil || subject.TenantID != tenantIDFromPath {
		h.logger.Error("Tenant ID mismatch between path and refresh token",
			zap.String("path_tenant_id", tenantIDFromPath),
			zap.String("token_tenant_id", func() string {
				if subject != nil {
					return subject.TenantID
				}
				return "<nil>"
			}()))
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

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

	// Generate new tokens with the same subject as the original token
	if subject == nil {
		h.logger.Error("Refresh token missing subject; cannot re-issue access token", zap.String("client_id", clientID))
		h.sendError(w, errors.ErrInvalidRefreshToken)
		return
	}

	accessToken, _, err := h.tokenGen.GenerateAccessToken(subject)
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
		Subject:   subject, // Preserve subject for future refreshes
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
