package requeuer

import (
	"context"
	"fmt"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/redis/go-redis/v9"
)

type RinhaRequeuer struct {
	redisClient   *redis.Client
	mainQueueName string
	stopChan      chan struct{}
}

func NewRinhaRequeuer(client *redis.Client, mainQueueName string) *RinhaRequeuer {
	return &RinhaRequeuer{
		redisClient:   client,
		mainQueueName: mainQueueName,
		stopChan:      make(chan struct{}),
	}
}

func (r *RinhaRequeuer) Start() {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				r.processDelayedItems()
			case <-r.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (r *RinhaRequeuer) Stop() {
	close(r.stopChan)
}

func (r *RinhaRequeuer) processDelayedItems() {
	ctx := context.Background()
	now := time.Now().Unix()
	maxScore := fmt.Sprintf("%d", now)

	items, err := r.redisClient.ZRangeByScore(ctx, env.Env.RedisDelayedQueueName, &redis.ZRangeBy{
		Min:   "0",
		Max:   maxScore,
		Count: 100,
	}).Result()
	if err != nil || len(items) == 0 {
		return
	}

	pipe := r.redisClient.Pipeline()
	for _, item := range items {
		pipe.LPush(ctx, r.mainQueueName, item)
	}

	pipe.ZRemRangeByScore(ctx, env.Env.RedisDelayedQueueName, "0", maxScore)
	pipe.Exec(ctx)
}
