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

	go wp.healthCheckRoutine()
	go wp.startNATSConsumer()
	go wp.startMaintenanceRoutines()
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

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	for {
		select {
		case job := <-wp.jobQueue:
			log.Printf("Worker %d processing payment for correlation ID: %s", id, job.Request.CorrelationId)
			if err := wp.markPaymentInProgress(job.Request.CorrelationId); err != nil {
				log.Printf("Worker %d: failed to mark payment in progress: %v", id, err)
			}

			response, err := wp.processPayment(client, job.Request)
			if err != nil {
				wp.cleanupInProgressMarker(job.Request.CorrelationId)
				select {
				case job.ErrorCh <- err:
				case <-time.After(1 * time.Second):
					log.Printf("Worker %d: timeout sending error response", id)
				}
				continue
			}

			select {
			case job.ResponseCh <- response:
			case <-time.After(5 * time.Second):
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

func (wp *WorkerPool) processPayment(client *http.Client, request *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	processor := wp.selectBestProcessor()

	paymentReq := PaymentProcessorRequest{
		CorrelationID: request.CorrelationId,
		Amount:        request.Amount,
		RequestedAt:   time.Now().UTC(),
	}

	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %v", err)
	}

	url := fmt.Sprintf("%s/payments", processor.URL)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		if processor.Name == "default" {
			log.Printf("Default processor failed, trying fallback for %s", request.CorrelationId)
			return wp.tryFallbackProcessor(client, request)
		}
		return nil, fmt.Errorf("payment processor request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		if processor.Name == "default" {
			log.Printf("Default processor returned %d, trying fallback for %s", resp.StatusCode, request.CorrelationId)
			return wp.tryFallbackProcessor(client, request)
		}
		return nil, fmt.Errorf("payment processor returned status %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payment processor returned status %d", resp.StatusCode)
	}
	err = wp.storePaymentData(processor.Name, request.Amount, request.CorrelationId)
	if err != nil {
		log.Printf("Failed to store payment data: %v", err)
	}
	var procResp PaymentProcessorResponse
	if err := json.NewDecoder(resp.Body).Decode(&procResp); err != nil {
		return nil, fmt.Errorf("failed to decode processor response: %v", err)
	}
	return &pb.PaymentResponse{
		Code:    int32(resp.StatusCode),
		Message: procResp.Message,
	}, nil
}

func (wp *WorkerPool) tryFallbackProcessor(client *http.Client, request *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	fallbackProcessor := PaymentProcessor{Name: "fallback", URL: "http://payment-processor-fallback:8080"}

	paymentReq := PaymentProcessorRequest{
		CorrelationID: request.CorrelationId,
		Amount:        request.Amount,
		RequestedAt:   time.Now().UTC(),
	}
	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fallback payment request: %v", err)
	}
	url := fmt.Sprintf("%s/payments", fallbackProcessor.URL)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fallback processor request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fallback processor returned status %d", resp.StatusCode)
	}
	err = wp.storePaymentData(fallbackProcessor.Name, request.Amount, request.CorrelationId)
	if err != nil {
		log.Printf("Failed to store fallback payment data: %v", err)
	}
	var procResp PaymentProcessorResponse
	if err := json.NewDecoder(resp.Body).Decode(&procResp); err != nil {
		return nil, fmt.Errorf("failed to decode fallback processor response: %v", err)
	}
	return &pb.PaymentResponse{
		Code:    int32(resp.StatusCode),
		Message: procResp.Message,
	}, nil
}

func (wp *WorkerPool) selectBestProcessor() PaymentProcessor {
	wp.healthMutex.RLock()
	defer wp.healthMutex.RUnlock()

	if health, exists := wp.healthCheckCache["default"]; exists && !health.Failing {
		return wp.processors[0]
	}
	if health, exists := wp.healthCheckCache["fallback"]; exists && !health.Failing {
		return wp.processors[1]
	}
	return wp.processors[0]
}

func (wp *WorkerPool) healthCheckRoutine() {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for {
		select {
		case <-ticker.C:
			for _, processor := range wp.processors {
				wp.checkProcessorHealth(client, processor)
			}
		case <-wp.quit:
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

func (wp *WorkerPool) storePaymentData(processorName string, amount float64, correlationId string) error {
	ctx := context.Background()
	pipe := wp.redisClient.TxPipeline()
	now := time.Now().UTC()
	paymentRecord := map[string]any{
		"correlationId": correlationId,
		"amount":        amount,
		"processor":     processorName,
		"processedAt":   now.Format(time.RFC3339),
		"timestamp":     now.Unix(),
	}

	recordKey := fmt.Sprintf("payment:%s", correlationId)
	paymentJSON, err := json.Marshal(paymentRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal payment record: %v", err)
	}

	pipe.Set(ctx, recordKey, paymentJSON, 0)
	listKey := fmt.Sprintf("payments:list:%s", processorName)
	pipe.ZAdd(ctx, listKey, &redis.Z{
		Score:  float64(now.Unix()),
		Member: correlationId,
	})

	countKey := fmt.Sprintf("payments:%s:count", processorName)
	pipe.Incr(ctx, countKey)
	amountKey := fmt.Sprintf("payments:%s:amount", processorName)
	pipe.IncrByFloat(ctx, amountKey, amount)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute payment storage transaction: %v", err)
	}
	return nil
}

func (wp *WorkerPool) startNATSConsumer() {
	_, err := wp.natsConn.Subscribe("payments.requests", func(msg *nats.Msg) {
		var request pb.PaymentRequest
		if err := json.Unmarshal(msg.Data, &request); err != nil {
			log.Printf("Failed to unmarshal NATS message: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		response, err := wp.SubmitJob(ctx, &request)
		if err != nil {
			log.Printf("Failed to process payment from NATS: %v", err)
			return
		}
		responseData, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			return
		}

		replySubject := fmt.Sprintf("payments.responses.%s", request.CorrelationId)
		wp.natsConn.Publish(replySubject, responseData)
	})
	if err != nil {
		log.Printf("Failed to subscribe to NATS: %v", err)
	}
}

type PaymentError struct {
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}

func (wp *WorkerPool) SubmitJob(ctx context.Context, request *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	responseCh := make(chan *pb.PaymentResponse, 1)
	errorCh := make(chan error, 1)

	job := PaymentJob{
		Request:    request,
		ResponseCh: responseCh,
		ErrorCh:    errorCh,
	}

	select {
	case wp.jobQueue <- job:
	case <-time.After(1 * time.Second):
		return nil, &PaymentError{Message: "Worker pool queue is full, request timeout"}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-time.After(30 * time.Second):
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

func (wp *WorkerPool) CheckCorrelationIdExists(ctx context.Context, correlationId string) (bool, error) {
	recordKey := fmt.Sprintf("payment:%s", correlationId)
	exists, err := wp.redisClient.Exists(ctx, recordKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check correlation ID existence: %v", err)
	}
	if exists > 0 {
		return true, nil
	}

	progressKey := fmt.Sprintf("payment:progress:%s", correlationId)
	inProgress, err := wp.redisClient.Exists(ctx, progressKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check correlation ID in progress: %v", err)
	}

	return inProgress > 0, nil
}

func (wp *WorkerPool) markPaymentInProgress(correlationId string) error {
	ctx := context.Background()
	progressKey := fmt.Sprintf("payment:progress:%s", correlationId)
	return wp.redisClient.SetNX(ctx, progressKey, time.Now().Unix(), 5*time.Minute).Err()
}

func (wp *WorkerPool) cleanupInProgressMarker(correlationId string) {
	ctx := context.Background()
	progressKey := fmt.Sprintf("payment:progress:%s", correlationId)
	wp.redisClient.Del(ctx, progressKey)
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
