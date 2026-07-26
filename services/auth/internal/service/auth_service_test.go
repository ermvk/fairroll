package service

import (
	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	"fairroll/pkg/models"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAuthToken(t *testing.T) {

	svc := &AuthService{
		jwtConfig: &config.JWTConfig{
			SecretKey:      "test-secret",
			AccessTokenTTL: 15 * time.Minute,
		},
		logger: logger.GetZap(),
	}

	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
	}

	tokenString, err := svc.generateAccessToken(user)

	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(svc.jwtConfig.SecretKey), nil
	})

	require.NoError(t, err)
	require.NotEmpty(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)

	assert.Equal(t, user.ID.String(), claims["sub"])
	assert.Equal(t, user.Email, claims["email"])
	assert.Equal(t, user.Username, claims["username"])
	assert.Equal(t, "access", claims["type"])

}
