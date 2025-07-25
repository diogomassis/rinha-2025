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
	queueName           string
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
		queueName:           env.Env.InstanceName,
		redisQueue:          redisQueue,
		redisPersistence:    redisPersistence,
		paymentOrchestrator: paymentOrchestrator,
		waitGroup:           &sync.WaitGroup{},
	}
}

func (rw *RinhaWorker) Start() {
	log.Printf("[worker] Starting %d workers for queue: %s", rw.numWorkers, rw.queueName)

	var ctx context.Context
	ctx, rw.cancelFunc = context.WithCancel(context.Background())

	for i := 1; i <= rw.numWorkers; i++ {
		rw.waitGroup.Add(i)
		go rw.worker(ctx, i)
	}
}

func (rw *RinhaWorker) Stop() {
	log.Println("[worker] Shutting down the worker pool...")

	if rw.cancelFunc != nil {
		rw.cancelFunc()
	}

	rw.waitGroup.Wait()
	log.Println("[worker] All workers have been safely shut down.")
}

func (rw *RinhaWorker) worker(ctx context.Context, id int) {
	defer rw.waitGroup.Done()
	log.Printf("[worker] Worker #%d initialized and waiting for jobs...", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker] Worker #%d received shutdown signal, exiting...", id)
			return
		default:
			data, err := rw.redisQueue.PopFromQueue(ctx, rw.queueName)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
					continue
				}
				log.Printf("[worker] Error in worker #%d while popping from queue: %v", id, err)
				continue
			}
			if err := rw.processPayment(ctx, *data); err != nil {
				log.Printf("[worker] Error in jobFunc in worker #%d: %v", id, err)
			}
		}
	}
}

const MAX_RETRIES = 3
const DELAYED_QUEUE_KEY = "payments:queue:delayed"
const DEAD_LETTER_QUEUE_KEY = "payments:queue:dead-letter"

func (rw *RinhaWorker) processPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	log.Printf("[worker] Processing payment %s (Attempt #%d)", data.CorrelationId, data.RetryCount+1)

	completedPayment, err := rw.paymentOrchestrator.ExecutePayment(ctx, data)
	if err != nil {
		log.Printf("[worker] Orchestrator failed for payment %s: %v", data.CorrelationId, err)
		return rw.handleFailedPayment(ctx, data)
	}

	if err := rw.redisPersistence.Add(ctx, *completedPayment); err != nil {
		log.Printf("[worker] CRITICAL: Payment %s processed but failed to save summary: %v", completedPayment.CorrelationID, err)
		return rw.handleFailedPayment(ctx, data)
	}

	log.Printf("[worker] Payment %s processed and saved successfully via %s.", completedPayment.CorrelationID, completedPayment.Type)
	return nil
}

func (rw *RinhaWorker) handleFailedPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	if data.RetryCount >= MAX_RETRIES {
		log.Printf("[worker] Payment %s exceeded max retries. Moving to Dead Letter Queue.", data.CorrelationId)
		return rw.redisQueue.AddToDeadLetterQueue(ctx, DEAD_LETTER_QUEUE_KEY, data)
	}

	data.RetryCount++
	delay := time.Duration(10*data.RetryCount) * time.Second
	retryAt := time.Now().Add(delay)
	log.Printf("[worker] Re-queueing payment %s for another attempt in %v.", data.CorrelationId, delay)
	return rw.redisQueue.AddToDelayedQueue(ctx, DELAYED_QUEUE_KEY, data, retryAt)
}
