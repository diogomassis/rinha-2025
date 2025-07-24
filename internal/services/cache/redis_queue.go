package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/redis/go-redis/v9"
)

type RinhaRedisQueueService struct {
	client *redis.Client
}

func NewRinhaRedisQueueService(client *redis.Client) *RinhaRedisQueueService {
	return &RinhaRedisQueueService{
		client: client,
	}
}

func (r *RinhaRedisQueueService) AddToQueue(ctx context.Context, queueName string, pendingPayment models.RinhaPendingPayment) error {
	data, err := json.Marshal(pendingPayment)
	if err != nil {
		return fmt.Errorf("[cache] failed to marshal pending payment: %w", err)
	}

	_, err = r.client.LPush(ctx, queueName, data).Result()
	if err != nil {
		return fmt.Errorf("[cache] failed to add to queue: %w", err)
	}
	return nil
}

func (r *RinhaRedisQueueService) PopFromQueue(ctx context.Context, queueName string) (*models.RinhaPendingPayment, error) {
	result, err := r.client.BRPop(ctx, 0, queueName).Result()
	if err != nil {
		return nil, fmt.Errorf("[cache] failed to pop from queue: %w", err)
	}
	if len(result) < 2 {
		return nil, fmt.Errorf("[cache] unexpected BRPop result: %v", result)
	}

	var pendingPayment models.RinhaPendingPayment
	if err := json.Unmarshal([]byte(result[1]), &pendingPayment); err != nil {
		return nil, fmt.Errorf("[cache] failed to unmarshal pending payment: %w", err)
	}
	return &pendingPayment, nil
}
