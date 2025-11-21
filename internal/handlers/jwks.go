package handlers

import (
	"encoding/json"
	"net/http"
	"session-service/internal/auth"

	"go.uber.org/zap"
)

// JWKSHandler handles JWKS endpoint requests
type JWKSHandler struct {
	keyManager *auth.KeyManager
	logger     *zap.Logger
}

// NewJWKSHandler creates a new JWKS handler
func NewJWKSHandler(keyManager *auth.KeyManager, logger *zap.Logger) *JWKSHandler {
	return &JWKSHandler{
		keyManager: keyManager,
		logger:     logger,
	}
}

// HandleJWKS handles GET /.well-known/jwks.json
func (h *JWKSHandler) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

