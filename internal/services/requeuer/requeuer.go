package requeuer

import (
	"context"
	"sync"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/rs/zerolog/log"
)

type RinhaRequeuer struct {
	retryPaymentChan   <-chan models.RinhaPendingPayment
	pendingPaymentChan chan<- models.RinhaPendingPayment
	waitGroup          *sync.WaitGroup
	cancelFunc         context.CancelFunc
}

func NewRinhaRequeuer(
	retryPaymentChan <-chan models.RinhaPendingPayment,
	pendingPaymentChan chan<- models.RinhaPendingPayment,
) *RinhaRequeuer {
	return &RinhaRequeuer{
		retryPaymentChan:   retryPaymentChan,
		pendingPaymentChan: pendingPaymentChan,
		waitGroup:          &sync.WaitGroup{},
	}
}

func (rr *RinhaRequeuer) Start() {
	log.Info().Msg("Starting payment requeuer...")
	var ctx context.Context
	ctx, rr.cancelFunc = context.WithCancel(context.Background())

	rr.waitGroup.Add(1)
	go rr.requeueWorker(ctx)
}

func (rr *RinhaRequeuer) Stop() {
	if rr.cancelFunc != nil {
		rr.cancelFunc()
	}
	rr.waitGroup.Wait()
}

func (rr *RinhaRequeuer) requeueWorker(ctx context.Context) {
	defer rr.waitGroup.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case paymentToRequeue := <-rr.retryPaymentChan:
			select {
			case rr.pendingPaymentChan <- paymentToRequeue:
			case <-ctx.Done():
				return
			}
		}
	}
}
