package handlers

import (
	"encoding/json"
	"net/http"
	"session-service/internal/auth"
	"session-service/internal/models"
	"session-service/pkg/errors"

	"go.uber.org/zap"
)

// VerifyHandler handles token verification requests
type VerifyHandler struct {
	validator *auth.TokenValidator
	logger    *zap.Logger
}

// NewVerifyHandler creates a new verify handler
func NewVerifyHandler(validator *auth.TokenValidator, logger *zap.Logger) *VerifyHandler {
	return &VerifyHandler{
		validator: validator,
		logger:    logger,
	}
}

// HandleVerify handles POST /oauth/verify
func (h *VerifyHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, errors.Wrap(err, errors.ErrInvalidToken))
		return
	}

	if req.Token == "" {
		h.sendError(w, errors.ErrInvalidToken)
		return
	}

	// Validate token
	claims, err := h.validator.ValidateToken(ctx, req.Token)
	if err != nil {
		h.logger.Debug("Token validation failed", zap.Error(err))
		h.sendResponse(w, http.StatusOK, &models.VerifyResponse{
			Valid:   false,
			Message: err.Error(),
		})
		return
	}

	// Convert claims to map[string]interface{}
	claimsMap := make(map[string]interface{})
	for k, v := range claims {
		claimsMap[k] = v
	}

	h.sendResponse(w, http.StatusOK, &models.VerifyResponse{
		Valid:  true,
		Claims: claimsMap,
	})
}

func (h *VerifyHandler) sendError(w http.ResponseWriter, err *errors.ServiceError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             err.Code,
		"error_description": err.Message,
	})
}

func (h *VerifyHandler) sendResponse(w http.ResponseWriter, status int, data *models.VerifyResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

