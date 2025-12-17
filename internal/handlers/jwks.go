package handlers

import (
	"encoding/json"
	"net/http"
	"session-service/internal/auth"
	"session-service/internal/database"
	"session-service/pkg/errors"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// JWKSHandler handles JWKS endpoint requests
type JWKSHandler struct {
	repo       database.Repository
	keyManager *auth.KeyManager
	logger     *zap.Logger
}

// NewJWKSHandler creates a new JWKS handler
func NewJWKSHandler(repo database.Repository, keyManager *auth.KeyManager, logger *zap.Logger) *JWKSHandler {
	return &JWKSHandler{
		repo:       repo,
		keyManager: keyManager,
		logger:     logger,
	}
}

// HandleJWKS handles GET /{tenant_id}/discovery/v1.0/keys
// @Summary     Get JSON Web Key Set (JWKS)
// @Description Returns the public keys in JWKS format for JWT validation. Supports key rotation with multiple active keys.
// @Tags        oidc
// @Param       tenant_id path string true "Tenant ID"
// @Produce     application/json
// @Success     200  {object}  map[string]interface{} "JWKS response"
// @Failure     500  {object}  map[string]string
// @Router      /{tenant_id}/discovery/v1.0/keys [get]
func (h *JWKSHandler) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract tenant_id from URL path and ensure it exists in the database.
	vars := mux.Vars(r)
	tenantID := vars["tenant_id"]
	if tenantID == "" {
		h.sendError(w, errors.ErrInvalidRequest)
		return
	}

	// Ensure tenant exists (no auto-create for discovery).
	if err := h.repo.EnsureTenantExists(r.Context(), tenantID); err != nil {
		h.logger.Error("Tenant does not exist for JWKS request", zap.String("tenant_id", tenantID), zap.Error(err))
		h.sendError(w, errors.Wrap(err, errors.ErrInvalidRequest))
		return
	}

	keySet := h.keyManager.GetJWKSet()

	// Marshal to JSON
	data, err := json.Marshal(keySet)
	if err != nil {
		h.logger.Error("Failed to marshal JWKS", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *JWKSHandler) sendError(w http.ResponseWriter, err *errors.ServiceError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             err.Code,
		"error_description": err.Message,
	})
}
