package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/diogomassis/rinha-2025/internal/models"
)

type PaymentProcessor interface {
	GetName() string
	ProcessPayment(ctx context.Context, payment *models.RinhaPendingPayment) error
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

func NewHTTPPaymentProcessor(name, url string) *HTTPPaymentProcessor {
	return &HTTPPaymentProcessor{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
			},
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewBuffer(jsonData))
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
