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

	// Clients
	GetClientByID(ctx context.Context, clientID string) (*models.Client, error)
	UpdateClientUpdatedAt(ctx context.Context, clientID string) error

	// Tenants & Users
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
	EnsureTenantExists(ctx context.Context, tenantID string) error
	UpsertUserAndRoles(ctx context.Context, user models.User, roles []string) error
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
		SELECT id, client_id, client_secret_hash, rate_limit, tenant_id, user_id, created_at, updated_at
		FROM clients
		WHERE client_id = $1
	`

	var client models.Client
	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&client.ID,
		&client.ClientID,
		&client.ClientSecretHash,
		&client.RateLimit,
		&client.TenantID,
		&client.UserID,
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

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
		SELECT id, tenant_id, email, full_name, phone_number, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	var email sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.TenantID,
		&email,
		&user.FullName,
		&user.PhoneNumber,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get user by ID", zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	// Convert NULL email to empty string
	if email.Valid {
		user.Email = email.String
	} else {
		user.Email = ""
	}

	return &user, nil
}

// GetUserRoles retrieves all roles for a given user
func (r *PostgresRepository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT role
		FROM user_roles
		WHERE user_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get user roles", zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			r.logger.Error("Failed to scan user role", zap.Error(err))
			return nil, err
		}
		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

// EnsureTenantExists verifies that a tenant with the given ID exists.
// It returns sql.ErrNoRows if the tenant does not exist so callers can map
// this to an appropriate invalid_request-style error.
func (r *PostgresRepository) EnsureTenantExists(ctx context.Context, tenantID string) error {
	query := `
		SELECT 1
		FROM tenants
		WHERE id = $1
	`

	var dummy int
	err := r.db.QueryRowContext(ctx, query, tenantID).Scan(&dummy)
	if err == sql.ErrNoRows {
		return err
	}
	if err != nil {
		r.logger.Error("Failed to ensure tenant exists", zap.String("tenant_id", tenantID), zap.Error(err))
		return err
	}

	return nil
}

// UpsertUserAndRoles upserts a user and, if roles are provided, replaces all
// role assignments for that user in a single transaction.
func (r *PostgresRepository) UpsertUserAndRoles(ctx context.Context, user models.User, roles []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("Failed to rollback transaction", zap.Error(rbErr))
			}
		}
	}()

	userQuery := `
		INSERT INTO users (id, tenant_id, email, full_name, phone_number)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5)
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id,
		    email = NULLIF(EXCLUDED.email, ''),
		    full_name = EXCLUDED.full_name,
		    phone_number = EXCLUDED.phone_number
	`

	// NULLIF in SQL converts empty strings to NULL, so empty email will be stored as NULL
	if _, err = tx.ExecContext(ctx, userQuery,
		user.ID,
		user.TenantID,
		user.Email,
		user.FullName,
		user.PhoneNumber,
	); err != nil {
		r.logger.Error("Failed to upsert user", zap.String("user_id", user.ID), zap.Error(err))
		return err
	}

	// If roles slice is non-nil, we treat it as authoritative and replace roles.
	if roles != nil {
		if _, err = tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, user.ID); err != nil {
			r.logger.Error("Failed to delete existing user roles", zap.String("user_id", user.ID), zap.Error(err))
			return err
		}

		if len(roles) > 0 {
			roleInsert := `
				INSERT INTO user_roles (user_id, role)
				VALUES ($1, $2)
				ON CONFLICT (user_id, role) DO NOTHING
			`
			for _, role := range roles {
				if _, err = tx.ExecContext(ctx, roleInsert, user.ID, role); err != nil {
					r.logger.Error("Failed to insert user role", zap.String("user_id", user.ID), zap.String("role", role), zap.Error(err))
					return err
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		r.logger.Error("Failed to commit user upsert transaction", zap.String("user_id", user.ID), zap.Error(err))
		return err
	}

	return nil
}
