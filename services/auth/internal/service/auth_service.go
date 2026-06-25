package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fairroll/pkg/config"
	"fairroll/pkg/errors"
	"fairroll/pkg/logger"
	"fairroll/pkg/models"
	"fairroll/services/auth/internal/repository"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	jwtConfig   *config.JWTConfig
	logger      *zap.Logger
}

func NewAuthService(userRepo *repository.UserRepository, sessionRepo *repository.SessionRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtConfig:   &cfg.JWT,
		logger:      logger.GetZap(),
	}
}

func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*models.User, error) {
	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && err != repository.ErrNotFound {
		return nil, errors.NewDatabaseError(err)
	}
	if existing != nil {
		return nil, errors.NewConflictError("Email already registered")
	}

	existingUser, err := s.userRepo.GetByUsername(ctx, req.UserName)
	if err != nil && err != repository.ErrNotFound {
		return nil, errors.NewDatabaseError(err)
	}

	if existingUser != nil {
		return nil, errors.NewConflictError("Username already taken")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, errors.NewInternalError(err)
	}

	user := &models.User{
		Email:        req.Email,
		Username:     req.UserName,
		PasswordHash: string(hashedPassword),
		KYCStatus:    "unverified",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	createdUser, err := s.userRepo.Create(ctx, user)
	if err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, errors.NewDatabaseError(err)
	}

	s.logger.Info("User registered successfuly", zap.String("user_id", createdUser.ID.String()), zap.String("email", createdUser.Email))

	return createdUser, nil
}

func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)

	if err != nil || user == nil {
		s.logger.Warn("Login failed: user not found", zap.String("email", req.Email))
		return nil, errors.NewInvalidCredentialsError("Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		s.logger.Warn("Login failed: invalid password", zap.String("user_id", user.ID.String()))
		return nil, errors.NewInvalidCredentialsError("Invalied email or password")
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	refreshToken, expiresAt, err := s.generateRefreshToken()
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, errors.NewDatabaseError(err)
	}

	s.logger.Info("User logged in successfully", zap.String("user_id", user.ID.String()), zap.String("email", user.Email))

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
		User: &models.User{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			KYCStatus: user.KYCStatus,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil || session == nil {
		return nil, errors.NewUnauthorizedError("Invalid refresh token")
	}

	if time.Now().After(session.ExpiresAt) {
		s.logger.Warn("Refresh token expired", zap.String("user_id", session.UserID.String()))
		return nil, errors.NewUnauthorizedError("Refresh token expired")
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, errors.NewNotFoundError("User")
	}

	newAccessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	s.logger.Info("Token refreshed", zap.String("user_id", user.ID.String()))

	return &AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtConfig.AccessTokenTTL.Seconds()),
		User: &models.User{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			KYCStatus: user.KYCStatus,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

// support funx
// Generate access token (JWT)
func (s *AuthService) generateAccessToken(user *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.jwtConfig.AccessTokenTTL)

	claims := jwt.MapClaims{
		"sub":      user.ID.String(),
		"email":    user.Email,
		"username": user.Username,
		"iat":      now.Unix(),
		"exp":      expiresAt.Unix(),
		"type":     "access",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtConfig.SecretKey))
	if err != nil {
		s.logger.Error("Failed to sign access token", zap.Error(err))
		return "", err
	}
	return tokenString, nil
}

func (s *AuthService) generateRefreshToken() (string, time.Time, error) {

	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		s.logger.Error("Failed to generate random bytes", zap.Error(err))
		return "", time.Time{}, err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(s.jwtConfig.RefreshTokenTTL)

	return token, expiresAt, nil
}

type RegisterRequest struct {
	Email    string
	UserName string
	Password string
}

type LoginRequest struct {
	Email    string
	Password string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	User         *models.User
}
