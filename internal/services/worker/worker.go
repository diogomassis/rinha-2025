package worker

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
)

type RinhaWorker struct {
	numWorkers          int
	redisQueue          *cache.RinhaRedisQueueService
	redisPersistence    *cache.RinhaRedisPersistenceService
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator
	mainQueueChannel    <-chan models.RinhaPendingPayment

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewRinhaWorker(
	numWorkers int,
	redisQueue *cache.RinhaRedisQueueService,
	redisPersistence *cache.RinhaRedisPersistenceService,
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator,
	mainQueueChannel <-chan models.RinhaPendingPayment,
) *RinhaWorker {
	return &RinhaWorker{
		numWorkers:          numWorkers,
		redisQueue:          redisQueue,
		redisPersistence:    redisPersistence,
		paymentOrchestrator: paymentOrchestrator,
		mainQueueChannel:    mainQueueChannel,
		waitGroup:           &sync.WaitGroup{},
	}
}

func (rw *RinhaWorker) Start() {
	log.Info().Int("numWorkers", rw.numWorkers).Str("queue", env.Env.RedisQueueName).Msg("Starting workers")

	var ctx context.Context
	ctx, rw.cancelFunc = context.WithCancel(context.Background())

	for i := 1; i <= rw.numWorkers; i++ {
		rw.waitGroup.Add(i)
		go rw.worker(ctx)
	}
}

func (rw *RinhaWorker) Stop() {
	if rw.cancelFunc != nil {
		rw.cancelFunc()
	}
	rw.waitGroup.Wait()
}

func (rw *RinhaWorker) worker(ctx context.Context) {
	defer rw.waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-rw.mainQueueChannel:
			if err := rw.processPayment(ctx, data); err != nil {
				log.Error().Err(err).Str("correlationId", data.CorrelationId).Msg("Error processing payment")
			}
		}
	}
}

func (rw *RinhaWorker) processPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	completedPayment, err := rw.paymentOrchestrator.ExecutePayment(ctx, data)
	if err != nil {
		return rw.handleFailedPayment(ctx, data)
	}
	rw.redisPersistence.Add(ctx, *completedPayment)
	return nil
}

func (rw *RinhaWorker) handleFailedPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	if data.RetryCount >= 3 {
		return rw.redisQueue.AddToDeadLetterQueue(ctx, data)
	}
	data.RetryCount++
	delay := time.Duration(1*data.RetryCount) * time.Millisecond
	retryAt := time.Now().Add(delay)
	return rw.redisQueue.AddToDelayedQueue(ctx, data, retryAt)
}
