package services

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisService interface {
	// Token Management
	StoreRefreshToken(ctx context.Context, userID, token string, expiry time.Duration) error
	GetRefreshToken(ctx context.Context, userID string) (string, error)
	DeleteRefreshToken(ctx context.Context, userID string) error
	BlacklistToken(ctx context.Context, token string, expiry time.Duration) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)

	// Rate Limiting
	IncrementRequestCount(ctx context.Context, key string, window time.Duration) (int, error)
}

type redisService struct {
	client *redis.Client
}

func NewRedisService(client *redis.Client) RedisService {
	return &redisService{client: client}
}

func (r *redisService) StoreRefreshToken(ctx context.Context, userID, token string, expiry time.Duration) error {
	return r.client.Set(ctx, "refresh:"+userID, token, expiry).Err()
}

func (r *redisService) GetRefreshToken(ctx context.Context, userID string) (string, error) {
	return r.client.Get(ctx, "refresh:"+userID).Result()
}

func (r *redisService) DeleteRefreshToken(ctx context.Context, userID string) error {
	return r.client.Del(ctx, "refresh:"+userID).Err()
}

func (r *redisService) BlacklistToken(ctx context.Context, token string, expiry time.Duration) error {
	return r.client.Set(ctx, "blacklist:"+token, "1", expiry).Err()
}

func (r *redisService) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	res, err := r.client.Exists(ctx, "blacklist:"+token).Result()
	return res > 0, err
}

func (r *redisService) IncrementRequestCount(ctx context.Context, key string, window time.Duration) (int, error) {
	// Using Redis transactions for atomic increment
	var count int
	err := r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Get current count
		current, err := tx.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			return err
		}

		// Start transaction
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Incr(ctx, key)
			if current == 0 {
				pipe.Expire(ctx, key, window)
			}
			return nil
		})
		return err
	}, key)

	if err != nil {
		return 0, err
	}

	// Get final count
	count, err = r.client.Get(ctx, key).Int()
	if err != nil {
		return 0, err
	}

	return count, nil
}
