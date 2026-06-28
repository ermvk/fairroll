package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"fairroll/pkg/logger"
	"fairroll/pkg/models"
)

var ErrNotFound = errors.New("not found")

type OutboxEvent struct {
	EventType string
	Payload   []byte
}

type UserRepository struct {
	db     *pgx.Conn
	logger *zap.Logger
}

func NewUserRepository(db *pgx.Conn) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger.GetZap(),
	}
}

// Create new user in DB
func (r *UserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	r.logger.Info("Creating user", zap.String("email", user.Email), zap.String("username", user.Username))

	query := `INSERT INTO users (email, username, password_hash, kyc_status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	err := r.db.QueryRow(ctx, query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.KYCStatus,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)
	if err != nil {
		r.logger.Error("Failed to create user", zap.Error(err), zap.String("email", user.Email))
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) CreateWithOutbox(ctx context.Context, user *models.User, event *OutboxEvent) (*models.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	query := `INSERT INTO users (email, username, password_hash, kyc_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	err = tx.QueryRow(ctx, query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.KYCStatus,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)
	if err != nil {
		r.logger.Error("Failed to create user", zap.Error(err), zap.String("email", user.Email))
		return nil, err
	}

	outboxQuery := `INSERT INTO outbox_events (aggregate_id, event_type, payload) VALUES ($1, $2, $3)`
	if _, err = tx.Exec(ctx, outboxQuery, user.ID, event.EventType, event.Payload); err != nil {
		r.logger.Error("Failed to write outbox event", zap.Error(err), zap.String("user_id", user.ID.String()))
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return user, nil
}

// Get user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, email,  username, password_hash, kyc_status, created_at, updated_at
	FROM users WHERE email = $1`

	var user models.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, userName string) (*models.User, error) {
	query := `SELECT id, email,  username, password_hash, kyc_status, created_at, updated_at
	FROM users WHERE username = $1`

	var user models.User
	err := r.db.QueryRow(ctx, query, userName).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get user by username", zap.Error(err), zap.String("username", userName))
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `SELECT id, email, username, password_hash, kyc_status, created_at, updated_at
		FROM users WHERE id = $1`

	var user models.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get user by user id", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `UPDATE users
	SET email = $1, username = $2, password_hash = $3, kyc_status = $4, updated_at = $5
	WHERE id = $6`

	_, err := r.db.Exec(ctx, query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.KYCStatus,
		time.Now(),
		user.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update user", zap.Error(err), zap.String("user_id", user.ID.String()))
		return err
	}

	return nil
}
