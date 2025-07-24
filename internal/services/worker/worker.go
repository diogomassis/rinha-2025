package worker

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/redis/go-redis/v9"
)

type RinhaWorker struct {
	numWorkers int
	queueName  string
	jobFunc    RinhaJobFunc
	client     *cache.RinhaRedisClient

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewRinhaWorker(numWorkers int, client *cache.RinhaRedisClient, jobFunc RinhaJobFunc) *RinhaWorker {
	return &RinhaWorker{
		numWorkers: numWorkers,
		queueName:  env.Env.InstanceName,
		jobFunc:    jobFunc,
		client:     client,
		waitGroup:  &sync.WaitGroup{},
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

func (rw *RinhaWorker) worker(ctx context.Context, id int) {
	defer rw.waitGroup.Done()
	log.Printf("[worker] Worker #%d initialized and waiting for jobs...", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker] Worker #%d received shutdown signal, exiting...", id)
			return
		default:
			data, err := rw.client.PopFromQueue(ctx, rw.queueName)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
					continue
				}
				log.Printf("[worker] Error in worker #%d while popping from queue: %v", id, err)
				continue
			}
			if err := rw.jobFunc(ctx, *data); err != nil {
				log.Printf("[worker] Error in jobFunc in worker #%d: %v", id, err)
			}
		}
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
