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

	// Mock expectations
	mockCache.On("GetClient", mock.Anything, clientID).Return(nil, nil).Once() // Cache miss
	mockRepo.On("GetClientByID", mock.Anything, clientID).Return(client, nil)
	mockCache.On("SetClient", mock.Anything, client, 15*time.Minute).Return(nil)
	mockCache.On("CheckRateLimit", mock.Anything, clientID, 100, time.Minute).Return(false, nil)
	mockCache.On("StoreRefreshToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*models.RefreshTokenData"), cfg.RefreshTokenExpiry).Return(nil)
	mockRepo.On("UpdateClientUpdatedAt", mock.Anything, clientID).Return(nil)

	// Create request
	form := url.Values{}
	form.Add("grant_type", "client_credentials")
	form.Add("client_id", clientID)
	form.Add("client_secret", clientSecret)

	req := httptest.NewRequest("POST", "/oauth/token", nil)
	req.PostForm = form
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
