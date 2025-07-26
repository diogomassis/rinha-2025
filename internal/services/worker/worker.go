package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
	"github.com/redis/go-redis/v9"
)

type RinhaWorker struct {
	numWorkers          int
	redisQueue          *cache.RinhaRedisQueueService
	redisPersistence    *cache.RinhaRedisPersistenceService
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewRinhaWorker(
	numWorkers int,
	redisQueue *cache.RinhaRedisQueueService,
	redisPersistence *cache.RinhaRedisPersistenceService,
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator,
) *RinhaWorker {
	return &RinhaWorker{
		numWorkers:          numWorkers,
		redisQueue:          redisQueue,
		redisPersistence:    redisPersistence,
		paymentOrchestrator: paymentOrchestrator,
		waitGroup:           &sync.WaitGroup{},
	}
}

func (rw *RinhaWorker) Start() {
	log.Printf("[worker] Starting %d workers for queue: %s", rw.numWorkers, env.Env.RedisQueueName)

	var ctx context.Context
	ctx, rw.cancelFunc = context.WithCancel(context.Background())

	for i := 1; i <= rw.numWorkers; i++ {
		rw.waitGroup.Add(i)
		go rw.worker(ctx, i)
	}
}

func (rw *RinhaWorker) Stop() {
	if rw.cancelFunc != nil {
		rw.cancelFunc()
	}

	rw.waitGroup.Wait()
}

func (rw *RinhaWorker) worker(ctx context.Context, id int) {
	defer rw.waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, err := rw.redisQueue.PopFromQueue(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
					continue
				}
				continue
			}
			if err := rw.processPayment(ctx, *data); err != nil {
				// Error already handled in processPayment
			}
		}
	}
}

func (rw *RinhaWorker) processPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	completedPayment, err := rw.paymentOrchestrator.ExecutePayment(ctx, data)
	if err != nil {
		return rw.handleFailedPayment(ctx, data)
	}

	if err := rw.redisPersistence.Add(ctx, *completedPayment); err != nil {
		return rw.handleFailedPayment(ctx, data)
	}

	return nil
}

func (rw *RinhaWorker) handleFailedPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	if data.RetryCount >= 3 {
		return rw.redisQueue.AddToDeadLetterQueue(ctx, data)
	}

	data.RetryCount++
	delay := time.Duration(10*data.RetryCount) * time.Second
	retryAt := time.Now().Add(delay)
	return rw.redisQueue.AddToDelayedQueue(ctx, data, retryAt)
}
