package worker

import (
	"errors"
	"sync"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
)

type RinhaWorkerBuilder struct {
	numWorkers          int
	redisPersistence    *cache.RinhaRedisPersistenceService
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator
	pendingPaymentChan  <-chan models.RinhaPendingPayment
	retryPaymentChan    chan<- models.RinhaPendingPayment
}

func NewRinhaWorkerBuilder() *RinhaWorkerBuilder {
	return &RinhaWorkerBuilder{}
}

func (b *RinhaWorkerBuilder) WithNumWorkers(numWorkers int) *RinhaWorkerBuilder {
	b.numWorkers = numWorkers
	return b
}

func (b *RinhaWorkerBuilder) WithRedisPersistence(persistence *cache.RinhaRedisPersistenceService) *RinhaWorkerBuilder {
	b.redisPersistence = persistence
	return b
}

func (b *RinhaWorkerBuilder) WithPaymentOrchestrator(orchestrator *orchestrator.RinhaPaymentOrchestrator) *RinhaWorkerBuilder {
	b.paymentOrchestrator = orchestrator
	return b
}

func (b *RinhaWorkerBuilder) WithPendingPaymentChannel(channel <-chan models.RinhaPendingPayment) *RinhaWorkerBuilder {
	b.pendingPaymentChan = channel
	return b
}

func (b *RinhaWorkerBuilder) WithRetryPaymentChannel(channel chan<- models.RinhaPendingPayment) *RinhaWorkerBuilder {
	b.retryPaymentChan = channel
	return b
}

func (b *RinhaWorkerBuilder) Build() (*RinhaWorker, error) {
	if b.numWorkers <= 0 {
		return nil, errors.New("number of workers must be positive")
	}
	if b.redisPersistence == nil {
		return nil, errors.New("Redis persistence service is required")
	}
	if b.paymentOrchestrator == nil {
		return nil, errors.New("payment orchestrator is required")
	}
	if b.pendingPaymentChan == nil {
		return nil, errors.New("pending payments channel is required")
	}
	if b.retryPaymentChan == nil {
		return nil, errors.New("retry channel is required")
	}

	return &RinhaWorker{
		numWorkers:          b.numWorkers,
		redisPersistence:    b.redisPersistence,
		paymentOrchestrator: b.paymentOrchestrator,
		pendingPaymentChan:  b.pendingPaymentChan,
		retryPaymentChan:    b.retryPaymentChan,
		waitGroup:           &sync.WaitGroup{},
	}, nil
}
