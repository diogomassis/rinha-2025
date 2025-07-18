package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	pb "github.com/diogomassis/rinha-2025/proto"
	"github.com/diogomassis/rinha-2025/validator"
	"github.com/go-redis/redis/v8"
	"github.com/nats-io/nats.go"
)

type PaymentProcessor struct {
	Name string
	URL  string
}

type PaymentProcessorRequest struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

type PaymentProcessorResponse struct {
	Message string `json:"message"`
}

type HealthCheckResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type PaymentJob struct {
	Request    *pb.PaymentRequest
	ResponseCh chan *pb.PaymentResponse
	ErrorCh    chan error
}

type WorkerPool struct {
	jobQueue    chan PaymentJob
	workerCount int
	wg          sync.WaitGroup
	quit        chan bool
}

func NewWorkerPool(workerCount, queueSize int) *WorkerPool {
	return &WorkerPool{
		jobQueue:    make(chan PaymentJob, queueSize),
		workerCount: workerCount,
		quit:        make(chan bool),
	}
}

func (wp *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	close(wp.quit)
	wp.wg.Wait()
	close(wp.jobQueue)
	log.Println("Worker pool stopped")
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case job := <-wp.jobQueue:
			log.Printf("Worker %d processing payment for correlation ID: %s", id, job.Request.CorrelationId)

			// Simulate payment processing time
			time.Sleep(time.Millisecond * 100)

			// Process the payment
			response := &pb.PaymentResponse{
				Code:    http.StatusOK,
				Message: "Payment processed successfully by worker " + string(rune(id+'0')) + ". Correlation Id: " + job.Request.CorrelationId,
			}

			// Send response back
			select {
			case job.ResponseCh <- response:
			case <-time.After(5 * time.Second):
				// Timeout sending response
				select {
				case job.ErrorCh <- &PaymentError{Message: "Timeout sending response"}:
				default:
				}
			}

		case <-wp.quit:
			log.Printf("Worker %d stopping", id)
			return
		}
	}
}

// PaymentError represents a payment processing error
type PaymentError struct {
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}

// SubmitJob submits a job to the worker pool and returns the response
func (wp *WorkerPool) SubmitJob(ctx context.Context, request *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	responseCh := make(chan *pb.PaymentResponse, 1)
	errorCh := make(chan error, 1)

	job := PaymentJob{
		Request:    request,
		ResponseCh: responseCh,
		ErrorCh:    errorCh,
	}

	// Submit job to queue with timeout
	select {
	case wp.jobQueue <- job:
		// Job submitted successfully
	case <-time.After(1 * time.Second):
		return nil, &PaymentError{Message: "Worker pool queue is full, request timeout"}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Wait for response with timeout
	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-time.After(10 * time.Second):
		return nil, &PaymentError{Message: "Payment processing timeout"}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
