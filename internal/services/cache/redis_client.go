package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/redis/go-redis/v9"
)

type RinhaRedisClient struct {
	client *redis.Client
}

func NewRinhaRedisClient() *RinhaRedisClient {
	return &RinhaRedisClient{
		client: redis.NewClient(&redis.Options{
			Addr:         env.Env.RedisAddr,
			Password:     "",
			DB:           0,
			PoolSize:     100,
			MinIdleConns: 10,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolTimeout:  4 * time.Second,
		}),
	}
}

func (r *RinhaRedisClient) Ping(ctx context.Context) error {
	_, err := r.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("[cache] failed to ping Redis:: %w", err)
	}
	return nil
}

func (r *RinhaRedisClient) Close(ctx context.Context) error {
	err := r.client.Close()
	if err != nil {
		return fmt.Errorf("[cache] failed to close Redis connection: %w", err)
	}
	return nil
}

func (r *RinhaRedisClient) AddToQueue(ctx context.Context, queueName, value string) error {
	_, err := r.client.LPush(ctx, queueName, value).Result()
	if err != nil {
		return fmt.Errorf("[cache] failed to add to queue: %w", err)
	}
	return nil
}

func (r *RinhaRedisClient) PopFromQueue(ctx context.Context, queueName string) ([]byte, error) {
	result, err := r.client.BRPop(ctx, 0, queueName).Result()
	if err != nil {
		return nil, fmt.Errorf("[cache] failed to pop from queue: %w", err)
	}
	if len(result) < 2 {
		return nil, fmt.Errorf("[cache] unexpected BRPop result: %v", result)
	}
	return []byte(result[1]), nil
}
