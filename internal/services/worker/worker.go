package worker

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/cache"
	"github.com/diogomassis/rinha-2025/internal/services/orchestrator"
)

type RinhaWorker struct {
	numWorkers          int
	redisPersistence    *cache.RinhaRedisPersistenceService
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator
	pendingPaymentChan  <-chan models.RinhaPendingPayment

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewRinhaWorker(
	numWorkers int,
	redisPersistence *cache.RinhaRedisPersistenceService,
	paymentOrchestrator *orchestrator.RinhaPaymentOrchestrator,
	pendingPaymentChan <-chan models.RinhaPendingPayment,
) *RinhaWorker {
	return &RinhaWorker{
		numWorkers:          numWorkers,
		redisPersistence:    redisPersistence,
		paymentOrchestrator: paymentOrchestrator,
		pendingPaymentChan:  pendingPaymentChan,
		waitGroup:           &sync.WaitGroup{},
	}
}

func (rw *RinhaWorker) Start() {
	log.Info().Int("numWorkers", rw.numWorkers).Msg("Starting workers")

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
		case data := <-rw.pendingPaymentChan:
			if err := rw.processPayment(ctx, data); err != nil {
				log.Error().Err(err).Str("correlationId", data.CorrelationId).Msg("Error processing payment")
			}
		}
	}
}

func (rw *RinhaWorker) processPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	completedPayment, err := rw.paymentOrchestrator.ExecutePayment(ctx, data)
	if err != nil {
		return err
	}
	rw.redisPersistence.Add(ctx, *completedPayment)
	return nil
}
