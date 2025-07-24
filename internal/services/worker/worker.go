package worker

import (
	"context"
	"sync"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
)

type RinhaJobFunc func(ctx context.Context, data []byte) error

type RinhaWorker struct {
	numWorkers int
	queueName  string
	jobFunc    RinhaJobFunc
	client     *cache.RinhaRedisClient

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewRinhaWorker(numWorkers int, jobFunc RinhaJobFunc) *RinhaWorker {
	return &RinhaWorker{
		numWorkers: numWorkers,
		queueName:  env.Env.InstanceName,
		jobFunc:    jobFunc,
		client:     cache.NewRinhaRedisClient(),
		waitGroup:  &sync.WaitGroup{},
	}
}
