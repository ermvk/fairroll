package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"fairroll/pkg/models"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRedisRepository struct {
	rdb *redis.Client
}

func NewSessionRedisRepository(rdb *redis.Client) *SessionRedisRepository {
	return &SessionRedisRepository{rdb: rdb}
}

func (r *SessionRedisRepository) Create(ctx context.Context, session *models.Session) error {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	key := "session:" + session.RefreshToken
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return errors.New("session expired")
	}

	if err := r.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return err
	}

	sessionIDKey := "session_id:" + session.ID.String()
	return r.rdb.Set(ctx, sessionIDKey, session.RefreshToken, ttl).Err()
}

func (r *SessionRedisRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	key := "session:" + refreshToken
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != redis.Nil {
		return nil, ErrSessionNotFound
	}

	if err != nil {
		return nil, err
	}

	var session models.Session
	if err = json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *SessionRedisRepository) Delete(ctx context.Context, sessionID uuid.UUID) error {
	key := "session_id:" + sessionID.String()
	refreshToken, err := r.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.rdb.Del(ctx, "session"+refreshToken).Err(); err != nil {
		return err
	}

	return r.rdb.Del(ctx, key).Err()
}

func (r *SessionRedisRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	iter := r.rdb.Scan(ctx, 0, "session:*", 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()

		data, err := r.rdb.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var session models.Session
		if err = json.Unmarshal(data, &session); err != nil {
			continue
		}
		if session.UserID == userID {
			r.rdb.Del(ctx, key)
		}
	}

	return iter.Err()
}
