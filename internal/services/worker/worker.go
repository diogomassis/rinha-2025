package worker

import (
	"fmt"
	"sync"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
	"github.com/diogomassis/rinha-2025/internal/services/persistence"
)

var (
	workerQuantity = 10
)

type Worker struct {
	paymentRequestCh chan *PaymentJob
	processors       map[string]*paymentprocessor.PaymentProcessor

	chooser *chooserchecker.ChooserService
	db      *persistence.PaymentPersistenceService

	waitGroup sync.WaitGroup
}

func New(chooser *chooserchecker.ChooserService, db *persistence.PaymentPersistenceService) *Worker {
	return &Worker{
		paymentRequestCh: make(chan *PaymentJob, 1000),
		chooser:          chooser,
		db:               db,
		processors: map[string]*paymentprocessor.PaymentProcessor{
			"d": paymentprocessor.New("d", env.Env.ProcessorDefaultUrl),
			"f": paymentprocessor.New("f", env.Env.ProcessorFallbackUrl),
		},
	}
}

func (w *Worker) Start() {
	for range workerQuantity {
		w.waitGroup.Add(1)
		go w.paymentProcessor()
	}
}

func (w *Worker) Stop() {
	close(w.paymentRequestCh)
	w.waitGroup.Wait()
}

func (w *Worker) paymentProcessor() {
	defer w.waitGroup.Done()
	for job := range w.paymentRequestCh {
		processorKey, err := w.chooser.ChooseNextService()
		if err != nil {
			continue
		}
		processor, ok := w.processors[processorKey]
		if !ok {
			continue
		}
		paymentResponse, err := processor.ProcessPayment(job.RequestPayment)
		if err != nil {
			continue
		}
		rowsAffected, err := w.db.SavePayment(paymentResponse)
		if err != nil || rowsAffected == 0 {
			go w.retryPersistPayment(paymentResponse) // <- Remember to review this retry logic
		}
	}
}

func (w *Worker) retryPersistPayment(paymentResponse *paymentprocessor.PaymentResponse) (int64, error) {
	var rowsAffected int64
	var err error
	for i := 0; i < 3; i++ {
		rowsAffected, err = w.db.SavePayment(paymentResponse)
		if err == nil && rowsAffected > 0 {
			return rowsAffected, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, err
}

func (w *Worker) AddPaymentJob(requestPayment *paymentprocessor.PaymentRequest) error {
	job := &PaymentJob{
		RequestPayment: requestPayment,
	}
	select {
	case w.paymentRequestCh <- job:
		return nil
	default:
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			select {
			case w.paymentRequestCh <- job:
				return nil
			default:
				time.Sleep(50 * time.Millisecond)
			}
		}
		return fmt.Errorf("failed to enqueue payment job after %d retries", maxRetries)
	}
}
