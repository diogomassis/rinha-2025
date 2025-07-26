package processor

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	json "github.com/json-iterator/go"

	"github.com/diogomassis/rinha-2025/internal/models"
)

type PaymentProcessor interface {
	GetName() string
	ProcessPayment(ctx context.Context, payment *models.RinhaPendingPayment) error
	CheckHealth() (*HealthStatus, error)
}

type PaymentRequest struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

type PaymentResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

type PaymentProcessorResponse struct {
	Message string `json:"message"`
}

type HTTPPaymentProcessor struct {
	name   string
	url    string
	client *http.Client
}

func NewHTTPPaymentProcessor(name, url string, timeout time.Duration) *HTTPPaymentProcessor {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &HTTPPaymentProcessor{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

func (p *HTTPPaymentProcessor) GetName() string {
	return p.name
}

func (p *HTTPPaymentProcessor) ProcessPayment(ctx context.Context, payment *models.RinhaPendingPayment) error {
	jsonData, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("failed to marshal payment request: %w", err)
	}

	paymentURL := fmt.Sprintf("%s/payments", p.url)
	req, err := http.NewRequest(http.MethodPost, paymentURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", p.name, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrServiceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: received status code %d", ErrServiceUnavailable, resp.StatusCode)
	}
	return fmt.Errorf("%w: received status code %d", ErrPaymentDefinitive, resp.StatusCode)
}

func (p *HTTPPaymentProcessor) CheckHealth() (*HealthStatus, error) {
	healthURL := fmt.Sprintf("%s/payments/service-health", p.url)
	req, err := http.NewRequest(http.MethodGet, healthURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned non-200 status: %d", resp.StatusCode)
	}
	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode health check response: %w", err)
	}
	return &status, nil
}
