package repository

import (
	"context"
	"database/sql"
	"errors"
	"fairroll/pkg/logger"
	"fairroll/pkg/models"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ErrNotFound = errors.New("not found")

type UserRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

type SessionRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger.GetZap(),
	}
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{
		db:     db,
		logger: logger.GetZap(),
	}
}

// Create new user in DB
func (r *UserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	r.logger.Info("Creating user", zap.String("email", user.Email), zap.String("username", user.Username))

	query := `INSERT INTO users (email, username, password_hash, kys_status, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.KYCStatus,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create user", zap.Error(err), zap.String("email", user.Email))
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		r.logger.Error("Failed  to get last insert id", zap.Error(err))
		return nil, err
	}

	user.ID = id
	return user, nil
}

// Get user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, email,  username, password_hash, kyc_status, created_at, updated_at
	FROM users WHERE email = ?`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
	}

	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, userName string) (*models.User, error) {
	query := `SELECT id, email,  username, password_hash, kyc_status, created_at, updated_at
	FROM users WHERE email = ?`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, userName).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Error("Failed to get user by username", zap.Error(err), zap.String("username", userName))
			return nil, err
		}
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, userID int64) (*models.User, error) {

	query := `SELECT id, email, username, password_hash, kyc_status, created_at, updated_at
		FROM users WHERE id = ?`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.KYCStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to get user by user id", zap.Error(err), zap.Int64("user_id", userID))
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `UPDATE users
	SET email = ?, username = ?, password_hash = ?, kyc_status = ?, updated_at = ?
	WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.KYCStatus,
		time.Now(),
		user.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update user", zap.Error(err), zap.Int64("user_id", user.ID))
		return err
	}

	return nil
}

func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	query := `INSERT INTO sessions (id, user_id, refresh_token, expires_at, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshToken,
		session.ExpiresAt,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create session", zap.Error(err), zap.Int64("user_id", session.UserID))
	}

	return nil
}

// Get session by refresh token
func (r *SessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	query := `SELECT id, user_id, refresh_token, expires_at, created_at, updated_at
		FROM sessions WHERE refresh_token = ?`

	var session models.Session
	err := r.db.QueryRowContext(ctx, query, refreshToken).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
			r.logger.Error("Failed to get session by refresh token", zap.Error(err))
			return nil, err
		}
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrNotFound
	}
	return &session, nil
}

// Delete session
func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, sessionID)

	if err != nil {
		r.logger.Error("Failed to delete session", zap.Error(err), zap.String("session_id", sessionID))
		return err
	}

	return nil
}

// Delete session by  user ID
func (r *UserRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE  FROM sessions WHERE user_id = ?`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete all sessions", zap.Error(err), zap.Int64("user_id", userID))
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	r.logger.Info("all sessions  deleted", zap.Int64("user_id", userID), zap.Int64("sessions_deleted", rowsAffected))
	return nil
}
