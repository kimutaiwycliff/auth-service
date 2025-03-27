package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisClient{client: rdb}
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	res, err := r.client.Exists(ctx, key).Result()
	return res > 0, err
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Ping checks the Redis connection
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Implement RedisService interface methods
func (r *RedisClient) StoreRefreshToken(ctx context.Context, userID, token string, expiry time.Duration) error {
	return r.Set(ctx, "refresh:"+userID, token, expiry)
}

func (r *RedisClient) GetRefreshToken(ctx context.Context, userID string) (string, error) {
	return r.Get(ctx, "refresh:"+userID)
}

func (r *RedisClient) DeleteRefreshToken(ctx context.Context, userID string) error {
	return r.Del(ctx, "refresh:"+userID)
}

func (r *RedisClient) BlacklistToken(ctx context.Context, token string, expiry time.Duration) error {
	return r.Set(ctx, "blacklist:"+token, "1", expiry)
}

func (r *RedisClient) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	return r.Exists(ctx, "blacklist:"+token)
}

func (r *RedisClient) IncrementRequestCount(ctx context.Context, key string, window time.Duration) (int, error) {
	pipe := r.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return int(incr.Val()), nil
}
