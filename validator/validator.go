package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type PaymentValidator struct {
	redisClient *redis.Client
	httpClient  *http.Client
}

type PaymentRecord struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	Processor     string  `json:"processor"`
	ProcessedAt   string  `json:"processedAt"`
	Timestamp     int64   `json:"timestamp"`
}

type ProcessorSummaryAdmin struct {
	TotalRequests     int64   `json:"totalRequests"`
	TotalAmount       float64 `json:"totalAmount"`
	TotalFee          float64 `json:"totalFee"`
	FeePerTransaction float64 `json:"feePerTransaction"`
}

func NewPaymentValidator(redisClient *redis.Client) *PaymentValidator {
	return &PaymentValidator{
		redisClient: redisClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (pv *PaymentValidator) ValidateConsistency() error {
	ctx := context.Background()
	ourSummary, err := pv.getOurSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to get our summary: %v", err)
	}

	defaultSummary, err := pv.getProcessorSummary("default")
	if err != nil {
		log.Printf("Warning: Could not get default processor summary: %v", err)
	}

	fallbackSummary, err := pv.getProcessorSummary("fallback")
	if err != nil {
		log.Printf("Warning: Could not get fallback processor summary: %v", err)
	}

	log.Printf("Consistency Check:")
	log.Printf("Our Default: %d requests, %.2f amount", ourSummary["default"].Requests, ourSummary["default"].Amount)
	log.Printf("Our Fallback: %d requests, %.2f amount", ourSummary["fallback"].Requests, ourSummary["fallback"].Amount)

	if defaultSummary != nil {
		log.Printf("Processor Default: %d requests, %.2f amount", defaultSummary.TotalRequests, defaultSummary.TotalAmount)
	}
	if fallbackSummary != nil {
		log.Printf("Processor Fallback: %d requests, %.2f amount", fallbackSummary.TotalRequests, fallbackSummary.TotalAmount)
	}
	return nil
}

func (pv *PaymentValidator) getOurSummary(ctx context.Context) (map[string]struct {
	Requests int64
	Amount   float64
}, error) {
	summary := make(map[string]struct {
		Requests int64
		Amount   float64
	})

	for _, processor := range []string{"default", "fallback"} {
		countKey := fmt.Sprintf("payments:%s:count", processor)
		count, err := pv.redisClient.Get(ctx, countKey).Int64()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to get count for %s: %v", processor, err)
		}

		amountKey := fmt.Sprintf("payments:%s:amount", processor)
		amount, err := pv.redisClient.Get(ctx, amountKey).Float64()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to get amount for %s: %v", processor, err)
		}

		summary[processor] = struct {
			Requests int64
			Amount   float64
		}{
			Requests: count,
			Amount:   amount,
		}
	}

	return summary, nil
}

func (pv *PaymentValidator) getProcessorSummary(processorName string) (*ProcessorSummaryAdmin, error) {
	baseURL := "http://payment-processor-default:8080"
	if processorName == "fallback" {
		baseURL = "http://payment-processor-fallback:8080"
	}

	url := fmt.Sprintf("%s/admin/payments-summary", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Rinha-Token", "123")

	resp, err := pv.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("processor returned status %d", resp.StatusCode)
	}
	var summary ProcessorSummaryAdmin
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (pv *PaymentValidator) CleanupOldRecords(olderThan time.Duration) error {
	ctx := context.Background()
	cutoff := time.Now().Add(-olderThan).Unix()
	for _, processor := range []string{"default", "fallback"} {
		listKey := fmt.Sprintf("payments:list:%s", processor)
		removed, err := pv.redisClient.ZRemRangeByScore(ctx, listKey, "-inf", fmt.Sprintf("%d", cutoff)).Result()
		if err != nil {
			log.Printf("Failed to cleanup old records for %s: %v", processor, err)
			continue
		}
		if removed > 0 {
			log.Printf("Cleaned up %d old payment records for %s processor", removed, processor)
		}
	}
	return nil
}
