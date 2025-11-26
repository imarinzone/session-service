package database

import (
	"context"
	"database/sql"
	"fmt"
	"session-service/internal/models"
	"time"

	"go.uber.org/zap"
	"gocloud.dev/postgres"
	_ "gocloud.dev/postgres/awspostgres"
	_ "gocloud.dev/postgres/gcppostgres"
)

// Repository defines the interface for database operations
type Repository interface {
	Close() error
	GetClientByID(ctx context.Context, clientID string) (*models.Client, error)
	UpdateClientUpdatedAt(ctx context.Context, clientID string) error
}

// PostgresRepository handles database operations
type PostgresRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewRepository creates a new repository instance
func NewRepository(ctx context.Context, databaseURL string, logger *zap.Logger) (Repository, error) {
	// Retry connection with exponential backoff
	var db *sql.DB
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = postgres.Open(ctx, databaseURL)
		if err == nil {
			// Test the connection
			if err = db.PingContext(ctx); err == nil {
				break
			}
			db.Close()
		}
		if i < maxRetries-1 {
			waitTime := time.Duration(i+1) * time.Second
			logger.Warn("Failed to connect to database, retrying...", zap.Int("attempt", i+1), zap.Duration("wait", waitTime), zap.Error(err))
			time.Sleep(waitTime)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}

	return &PostgresRepository{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// GetClientByID retrieves a client by client_id
func (r *PostgresRepository) GetClientByID(ctx context.Context, clientID string) (*models.Client, error) {
	query := `
		SELECT id, client_id, client_secret_hash, rate_limit, created_at, updated_at
		FROM clients
		WHERE client_id = $1
	`

	var client models.Client
	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&client.ID,
		&client.ClientID,
		&client.ClientSecretHash,
		&client.RateLimit,
		&client.CreatedAt,
		&client.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get client by ID", zap.String("client_id", clientID), zap.Error(err))
		return nil, err
	}

	return &client, nil
}

// UpdateClientUpdatedAt updates the updated_at timestamp for a client
func (r *PostgresRepository) UpdateClientUpdatedAt(ctx context.Context, clientID string) error {
	query := `UPDATE clients SET updated_at = $1 WHERE client_id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), clientID)
	if err != nil {
		r.logger.Error("Failed to update client updated_at", zap.String("client_id", clientID), zap.Error(err))
		return err
	}
	return nil
}
