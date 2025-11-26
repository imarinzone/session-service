package mocks

import (
	"context"
	"session-service/internal/models"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of database.Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRepository) GetClientByID(ctx context.Context, clientID string) (*models.Client, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Client), args.Error(1)
}

func (m *MockRepository) UpdateClientUpdatedAt(ctx context.Context, clientID string) error {
	args := m.Called(ctx, clientID)
	return args.Error(0)
}

// MockCache is a mock implementation of cache.Cache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCache) GetClient(ctx context.Context, clientID string) (*models.Client, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Client), args.Error(1)
}

func (m *MockCache) SetClient(ctx context.Context, client *models.Client, ttl time.Duration) error {
	args := m.Called(ctx, client, ttl)
	return args.Error(0)
}

func (m *MockCache) CheckRateLimit(ctx context.Context, clientID string, limit int, window time.Duration) (bool, error) {
	args := m.Called(ctx, clientID, limit, window)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) StoreRefreshToken(ctx context.Context, tokenID string, data *models.RefreshTokenData, ttl time.Duration) error {
	args := m.Called(ctx, tokenID, data, ttl)
	return args.Error(0)
}

func (m *MockCache) GetRefreshToken(ctx context.Context, tokenID string) (*models.RefreshTokenData, error) {
	args := m.Called(ctx, tokenID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RefreshTokenData), args.Error(1)
}

func (m *MockCache) DeleteRefreshToken(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

func (m *MockCache) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	args := m.Called(ctx, jti, ttl)
	return args.Error(0)
}

func (m *MockCache) RevokeRefreshToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	args := m.Called(ctx, tokenID, ttl)
	return args.Error(0)
}

func (m *MockCache) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	args := m.Called(ctx, jti)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) IsRefreshTokenRevoked(ctx context.Context, tokenID string) (bool, error) {
	args := m.Called(ctx, tokenID)
	return args.Bool(0), args.Error(1)
}
