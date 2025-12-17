package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"session-service/internal/auth"
	"session-service/internal/config"
	"session-service/internal/handlers"
	"session-service/internal/models"
	"session-service/test/helpers"
	"session-service/test/mocks"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func TestHandleToken_ClientCredentials(t *testing.T) {
	// Setup
	mockRepo := new(mocks.MockRepository)
	mockCache := new(mocks.MockCache)
	logger := zap.NewNop()

	privKey, pubKey := helpers.GenerateTestPEMKeys(t)
	km, err := auth.NewKeyManager(privKey, pubKey)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	tokenGen := auth.NewTokenGenerator(km, "issuer", "audience", 1*time.Hour, 32)
	tokenValidator := auth.NewTokenValidator(km, "issuer", "audience", mockCache)

	cfg := &config.Config{
		JWTExpiry:          1 * time.Hour,
		RefreshTokenExpiry: 24 * time.Hour,
	}

	handler := handlers.NewTokenHandler(mockRepo, mockCache, tokenGen, tokenValidator, cfg, logger)

	// Prepare test data
	clientID := "test-client"
	clientSecret := "test-secret"
	hashedSecret, _ := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)

	client := &models.Client{
		ClientID:         clientID,
		ClientSecretHash: string(hashedSecret),
		RateLimit:        100,
	}

	// User + tenant fields required by the client_credentials flow
	userID := "user-123"
	tenantID := "tenant-abc"

	// Existing user record
	existingUser := &models.User{
		ID:       userID,
		TenantID: tenantID,
	}

	roles := []string{"tenant-admin", "reader"}

	// Mock expectations
	mockCache.On("GetClient", mock.Anything, clientID).Return(nil, nil).Once() // Cache miss
	mockRepo.On("GetClientByID", mock.Anything, clientID).Return(client, nil)
	mockCache.On("SetClient", mock.Anything, client, 15*time.Minute).Return(nil)
	mockCache.On("CheckRateLimit", mock.Anything, clientID, 100, time.Minute).Return(false, nil)

	// Tenant must exist
	mockRepo.On("EnsureTenantExists", mock.Anything, tenantID).Return(nil)
	// User must already exist for client_credentials
	mockRepo.On("GetUserByID", mock.Anything, userID).Return(existingUser, nil)
	// Roles fetched from DB
	mockRepo.On("GetUserRoles", mock.Anything, userID).Return(roles, nil)

	mockCache.On("StoreRefreshToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*models.RefreshTokenData"), cfg.RefreshTokenExpiry).Return(nil)
	mockRepo.On("UpdateClientUpdatedAt", mock.Anything, clientID).Return(nil)

	// Create request with tenant in path and required user_id
	form := url.Values{}
	form.Add("grant_type", "client_credentials")
	form.Add("client_id", clientID)
	form.Add("client_secret", clientSecret)
	form.Add("user_id", userID)

	req := httptest.NewRequest("POST", "/"+tenantID+"/oauth2/v2.0/token", nil)
	req.PostForm = form
	// Set mux vars so handler can extract tenant_id from URL path
	req = mux.SetURLVars(req, map[string]string{"tenant_id": tenantID})

	rr := httptest.NewRecorder()

	// Execute
	handler.HandleToken(rr, req)

	// Verify
	assert.Equal(t, http.StatusOK, rr.Code)

	var response models.TokenResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, "Bearer", response.TokenType)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
