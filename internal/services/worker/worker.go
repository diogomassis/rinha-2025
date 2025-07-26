package worker

import (
	"context"
	"errors"
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
	retryPaymentChan    chan<- models.RinhaPendingPayment

	waitGroup  *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func (rw *RinhaWorker) Start() {
	log.Info().Int("numWorkers", rw.numWorkers).Msg("Starting workers")

	var ctx context.Context
	ctx, rw.cancelFunc = context.WithCancel(context.Background())

	for i := 1; i <= rw.numWorkers; i++ {
		rw.waitGroup.Add(1)
		go rw.worker(ctx)
	}
}

func (rw *RinhaWorker) Stop() {
	if rw.cancelFunc != nil {
		rw.cancelFunc()
	}
	rw.waitGroup.Wait()
}

func (rw *RinhaWorker) Wait() {
	rw.waitGroup.Wait()
}

func (rw *RinhaWorker) worker(ctx context.Context) {
	defer rw.waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-rw.pendingPaymentChan:
			if !ok { // Canal foi fechado
				return
			}
			if err := rw.processPayment(ctx, data); err != nil {
				if !errors.Is(err, orchestrator.AllHealthyPaymentFailed) &&
					!errors.Is(err, orchestrator.NoHealthyPaymentAvailable) {
					log.Error().Err(err).Str("correlationId", data.CorrelationId).Msg("Definitive error processing payment")
				}
			}
		}
	}
}

func (rw *RinhaWorker) processPayment(ctx context.Context, data models.RinhaPendingPayment) error {
	completedPayment, err := rw.paymentOrchestrator.ExecutePayment(ctx, data)
	if err != nil {
		if errors.Is(err, orchestrator.AllHealthyPaymentFailed) || errors.Is(err, orchestrator.NoHealthyPaymentAvailable) {
			select {
			case rw.retryPaymentChan <- data:
			case <-ctx.Done():
			}
		}
		return err
	}
	rw.redisPersistence.Add(ctx, *completedPayment)
	return nil
}
