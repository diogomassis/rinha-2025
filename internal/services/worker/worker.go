package worker

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
	"github.com/diogomassis/rinha-2025/internal/services/persistence"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	workerQuantity = 30
	ErrQueueFull   = errors.New("the processing queue is full")
)

type Worker struct {
	paymentRequestCh chan *PaymentJob
	healthChecker    *healthchecker.HealthChecker
	processors       map[string]*paymentprocessor.PaymentProcessor
	db               *persistence.PaymentPersistenceService

	waitGroup sync.WaitGroup
}

func New(healthChecker *healthchecker.HealthChecker, db *persistence.PaymentPersistenceService) *Worker {
	return &Worker{
		paymentRequestCh: make(chan *PaymentJob, 10000),
		healthChecker:    healthChecker,
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
		maxRetries := 5
		baseBackoff := 100 * time.Millisecond

		for i := 0; i < maxRetries; i++ {
			if i > 0 {
				time.Sleep(baseBackoff)
				baseBackoff *= 2
			}

			processorKey, err := w.healthChecker.ChooseNextService()
			if err != nil {
				log.Printf("WARN: No healthy service. Attempt %d/%d for %s.", i+1, maxRetries, job.RequestPayment.CorrelationID)
				continue
			}

			processor := w.processors[processorKey]

			startTime := time.Now()
			job.RequestPayment.RequestedAt = startTime.UTC().Format("2006-01-02T15:04:05.000Z")
			paymentResponse, err := processor.ProcessPayment(job.RequestPayment)
			duration := time.Since(startTime)

			if err != nil {
				w.healthChecker.RegisterServiceFailure(processorKey)
				if _, ok := err.(net.Error); ok {
					log.Printf("WARN: Timeout on attempt %d for %s.", i+1, job.RequestPayment.CorrelationID)
				}
				continue
			}
			paymentResponse.RequestedAt = job.RequestPayment.RequestedAt
			w.healthChecker.RegisterServiceSuccess(processorKey, duration)
			_, dbErr := w.db.SavePayment(paymentResponse)
			if dbErr != nil {
				var pgErr *pgconn.PgError
				if errors.As(dbErr, &pgErr) && pgErr.Code == "23505" {
					log.Printf("INFO: Payment %s already exists in the database (idempotency).", paymentResponse.CorrelationID)
				} else {
					log.Printf("CRITICAL: Failed to save payment %s: %v", paymentResponse.CorrelationID, dbErr)
				}
			}
			goto nextJob
		}
		log.Printf("CRITICAL: Job for %s discarded after %d attempts.", job.RequestPayment.CorrelationID, maxRetries)
	nextJob:
	}
}

func (w *Worker) AddPaymentJob(requestPayment *paymentprocessor.PaymentRequest) error {
	job := &PaymentJob{
		RequestPayment: requestPayment,
	}
	select {
	case w.paymentRequestCh <- job:
		return nil
	default:
		return ErrQueueFull
	}
}
