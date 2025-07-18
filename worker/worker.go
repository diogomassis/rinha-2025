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
	jobQueue         chan PaymentJob
	workerCount      int
	wg               sync.WaitGroup
	quit             chan bool
	processors       []PaymentProcessor
	redisClient      *redis.Client
	natsConn         *nats.Conn
	healthCheckCache map[string]*HealthCheckResponse
	healthMutex      sync.RWMutex
	lastHealthCheck  map[string]time.Time
	validator        *validator.PaymentValidator
}

func NewWorkerPool(workerCount, queueSize int, redisClient *redis.Client, natsConn *nats.Conn) *WorkerPool {
	processors := []PaymentProcessor{
		{Name: "default", URL: "http://payment-processor-default:8080"},
		{Name: "fallback", URL: "http://payment-processor-fallback:8080"},
	}

	return &WorkerPool{
		jobQueue:         make(chan PaymentJob, queueSize),
		workerCount:      workerCount,
		quit:             make(chan bool),
		processors:       processors,
		redisClient:      redisClient,
		natsConn:         natsConn,
		healthCheckCache: make(map[string]*HealthCheckResponse),
		lastHealthCheck:  make(map[string]time.Time),
		validator:        validator.NewPaymentValidator(redisClient),
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

func (wp *WorkerPool) checkProcessorHealth(client *http.Client, processor PaymentProcessor) {
	wp.healthMutex.Lock()
	lastCheck, exists := wp.lastHealthCheck[processor.Name]
	if exists && time.Since(lastCheck) < 5*time.Second {
		wp.healthMutex.Unlock()
		return
	}
	wp.lastHealthCheck[processor.Name] = time.Now()
	wp.healthMutex.Unlock()

	url := fmt.Sprintf("%s/payments/service-health", processor.URL)
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Health check failed for %s: %v", processor.Name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		log.Printf("Rate limited on health check for %s", processor.Name)
		return
	}

	var health HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		log.Printf("Failed to decode health response for %s: %v", processor.Name, err)
		return
	}

	wp.healthMutex.Lock()
	wp.healthCheckCache[processor.Name] = &health
	wp.healthMutex.Unlock()

	log.Printf("Health check for %s: failing=%t, minResponseTime=%dms",
		processor.Name, health.Failing, health.MinResponseTime)
}
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

func (wp *WorkerPool) GetPaymentsSummary(from, to string) (*pb.PaymentsSummaryResponse, error) {
	ctx := context.Background()

	response := &pb.PaymentsSummaryResponse{
		Default:  &pb.ProcessorSummary{},
		Fallback: &pb.ProcessorSummary{},
	}

	var fromTime, toTime int64
	if from != "" {
		if parsedFrom, err := time.Parse(time.RFC3339, from); err == nil {
			fromTime = parsedFrom.Unix()
		}
	}
	if to != "" {
		if parsedTo, err := time.Parse(time.RFC3339, to); err == nil {
			toTime = parsedTo.Unix()
		}
	}

	for _, processorName := range []string{"default", "fallback"} {
		var totalRequests int64
		var totalAmount float64

		if fromTime > 0 || toTime > 0 {
			listKey := fmt.Sprintf("payments:list:%s", processorName)

			min := "-inf"
			max := "+inf"
			if fromTime > 0 {
				min = fmt.Sprintf("%d", fromTime)
			}
			if toTime > 0 {
				max = fmt.Sprintf("%d", toTime)
			}

			correlationIds, err := wp.redisClient.ZRangeByScore(ctx, listKey, &redis.ZRangeBy{
				Min: min,
				Max: max,
			}).Result()

			if err != nil && err != redis.Nil {
				log.Printf("Failed to get payments in range for %s: %v", processorName, err)
				continue
			}

			for _, correlationId := range correlationIds {
				recordKey := fmt.Sprintf("payment:%s", correlationId)
				recordJSON, err := wp.redisClient.Get(ctx, recordKey).Result()
				if err != nil {
					continue
				}

				var record map[string]interface{}
				if err := json.Unmarshal([]byte(recordJSON), &record); err != nil {
					continue
				}

				if record["processor"] == processorName {
					totalRequests++
					if amount, ok := record["amount"].(float64); ok {
						totalAmount += amount
					}
				}
			}
		} else {
			countKey := fmt.Sprintf("payments:%s:count", processorName)
			count, err := wp.redisClient.Get(ctx, countKey).Int64()
			if err != nil && err != redis.Nil {
				log.Printf("Failed to get count for %s: %v", processorName, err)
			} else {
				totalRequests = count
			}

			amountKey := fmt.Sprintf("payments:%s:amount", processorName)
			amount, err := wp.redisClient.Get(ctx, amountKey).Float64()
			if err != nil && err != redis.Nil {
				log.Printf("Failed to get amount for %s: %v", processorName, err)
			} else {
				totalAmount = amount
			}
		}

		if processorName == "default" {
			response.Default.TotalRequests = totalRequests
			response.Default.TotalAmount = totalAmount
		} else {
			response.Fallback.TotalRequests = totalRequests
			response.Fallback.TotalAmount = totalAmount
		}
	}

	return response, nil
}
func (wp *WorkerPool) startMaintenanceRoutines() {
	validationTicker := time.NewTicker(5 * time.Minute)
	defer validationTicker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-validationTicker.C:
			if err := wp.validator.ValidateConsistency(); err != nil {
				log.Printf("Validation check failed: %v", err)
			}
		case <-cleanupTicker.C:
			if err := wp.validator.CleanupOldRecords(24 * time.Hour); err != nil {
				log.Printf("Cleanup failed: %v", err)
			}
		case <-wp.quit:
			return
		}
	}
}
