package database

import (
	"context"
	"database/sql"
	"session-service/internal/models"
	"time"

	"go.uber.org/zap"
	"gocloud.dev/postgres"
	_ "gocloud.dev/postgres/awspostgres"
	_ "gocloud.dev/postgres/gcppostgres"
)

// Repository handles database operations
type Repository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewRepository creates a new repository instance
func NewRepository(ctx context.Context, databaseURL string, logger *zap.Logger) (*Repository, error) {
	db, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &Repository{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}

// GetClientByID retrieves a client by client_id
func (r *Repository) GetClientByID(ctx context.Context, clientID string) (*models.Client, error) {
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
func (r *Repository) UpdateClientUpdatedAt(ctx context.Context, clientID string) error {
	query := `UPDATE clients SET updated_at = $1 WHERE client_id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), clientID)
	if err != nil {
		r.logger.Error("Failed to update client updated_at", zap.String("client_id", clientID), zap.Error(err))
		return err
	}
	return nil
}

