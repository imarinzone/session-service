package cache

import (
	"context"
	"encoding/json"
	"session-service/internal/models"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache handles Redis operations
type Cache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewCache creates a new cache instance
func NewCache(redisURL string, logger *zap.Logger) (*Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	// Test the connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Cache{
		client: client,
		logger: logger,
	}, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	return c.client.Close()
}

// GetClient retrieves client metadata from cache
func (c *Cache) GetClient(ctx context.Context, clientID string) (*models.Client, error) {
	key := "client:" + clientID
	data, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		c.logger.Error("Failed to get client from cache", zap.String("client_id", clientID), zap.Error(err))
		return nil, err
	}

	var client models.Client
	if err := json.Unmarshal([]byte(data), &client); err != nil {
		c.logger.Error("Failed to unmarshal client data", zap.Error(err))
		return nil, err
	}

	return &client, nil
}

// SetClient stores client metadata in cache
func (c *Cache) SetClient(ctx context.Context, client *models.Client, ttl time.Duration) error {
	key := "client:" + client.ClientID
	data, err := json.Marshal(client)
	if err != nil {
		return err
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		c.logger.Error("Failed to set client in cache", zap.String("client_id", client.ClientID), zap.Error(err))
		return err
	}

	return nil
}

// CheckRateLimit checks if the client has exceeded rate limit
func (c *Cache) CheckRateLimit(ctx context.Context, clientID string, limit int, window time.Duration) (bool, error) {
	key := "rate_limit:" + clientID
	count, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to increment rate limit counter", zap.String("client_id", clientID), zap.Error(err))
		return false, err
	}

	// Set expiration on first request
	if count == 1 {
		if err := c.client.Expire(ctx, key, window).Err(); err != nil {
			c.logger.Error("Failed to set rate limit expiration", zap.Error(err))
		}
	}

	return count > int64(limit), nil
}

// StoreRefreshToken stores a refresh token in Redis
func (c *Cache) StoreRefreshToken(ctx context.Context, tokenID string, data *models.RefreshTokenData, ttl time.Duration) error {
	key := "refresh_token:" + tokenID
	tokenData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := c.client.Set(ctx, key, tokenData, ttl).Err(); err != nil {
		c.logger.Error("Failed to store refresh token", zap.Error(err))
		return err
	}

	return nil
}

// GetRefreshToken retrieves refresh token data from Redis
func (c *Cache) GetRefreshToken(ctx context.Context, tokenID string) (*models.RefreshTokenData, error) {
	key := "refresh_token:" + tokenID
	data, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		c.logger.Error("Failed to get refresh token", zap.Error(err))
		return nil, err
	}

	var tokenData models.RefreshTokenData
	if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
		c.logger.Error("Failed to unmarshal refresh token data", zap.Error(err))
		return nil, err
	}

	return &tokenData, nil
}

// DeleteRefreshToken deletes a refresh token from Redis
func (c *Cache) DeleteRefreshToken(ctx context.Context, tokenID string) error {
	key := "refresh_token:" + tokenID
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Error("Failed to delete refresh token", zap.Error(err))
		return err
	}
	return nil
}

// RevokeToken adds a token to the revocation list
func (c *Cache) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	key := "revoked:jti:" + jti
	if err := c.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		c.logger.Error("Failed to revoke token", zap.String("jti", jti), zap.Error(err))
		return err
	}
	return nil
}

// RevokeRefreshToken adds a refresh token to the revocation list
func (c *Cache) RevokeRefreshToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	key := "revoked:refresh:" + tokenID
	if err := c.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		c.logger.Error("Failed to revoke refresh token", zap.String("token_id", tokenID), zap.Error(err))
		return err
	}
	return nil
}

// IsTokenRevoked checks if a token is revoked
func (c *Cache) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	key := "revoked:jti:" + jti
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to check token revocation", zap.String("jti", jti), zap.Error(err))
		return false, err
	}
	return exists > 0, nil
}

// IsRefreshTokenRevoked checks if a refresh token is revoked
func (c *Cache) IsRefreshTokenRevoked(ctx context.Context, tokenID string) (bool, error) {
	key := "revoked:refresh:" + tokenID
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to check refresh token revocation", zap.String("token_id", tokenID), zap.Error(err))
		return false, err
	}
	return exists > 0, nil
}
