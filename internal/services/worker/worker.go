package worker

import (
	"sync"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/diogomassis/rinha-2025/internal/persistence"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
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
		paymentResponse, err := processor.ProcessPayment(job.requestPayment)
		if err != nil {
			continue
		}
		err = w.db.SavePayment(paymentResponse)
		if err != nil {
			continue
		}
	}
}

func (w *Worker) AddPaymentJob(requestPayment *paymentprocessor.PaymentRequest) {
	job := &PaymentJob{
		requestPayment: requestPayment,
	}
	select {
	case w.paymentRequestCh <- job:
	default:
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			select {
			case w.paymentRequestCh <- job:
				return
			default:
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}
