package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PaymentProcessor interface {
	ProcessPayment(request *PaymentRequest) (*PaymentResponse, error)
	GetName() string
	GetURL() string
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

func (p *HTTPPaymentProcessor) GetURL() string {
	return p.url
}

func (p *HTTPPaymentProcessor) ProcessPayment(request *PaymentRequest) (*PaymentResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %v", err)
	}
	url := fmt.Sprintf("%s/payments", p.url)
	resp, err := p.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("payment processor request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("payment processor returned status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payment processor returned status %d", resp.StatusCode)
	}
	var procResp PaymentProcessorResponse
	if err := json.NewDecoder(resp.Body).Decode(&procResp); err != nil {
		return nil, fmt.Errorf("failed to decode processor response: %v", err)
	}

	return &PaymentResponse{
		Code:    int32(resp.StatusCode),
		Message: procResp.Message,
	}, nil
}
