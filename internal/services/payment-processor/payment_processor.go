package paymentprocessor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PaymentProcessor struct {
	name   string
	url    string
	client *http.Client
}

func New(name, url string) *PaymentProcessor {
	return &PaymentProcessor{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *PaymentProcessor) GetName() string {
	return p.name
}

func (p *PaymentProcessor) GetURL() string {
	return p.url
}

func (p *PaymentProcessor) ProcessPayment(request *PaymentRequest) (*PaymentResponse, error) {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("payment processor returned non-2xx status %d", resp.StatusCode)
	}
	return &PaymentResponse{
		CorrelationID: request.CorrelationID,
		Amount:        request.Amount,
		RequestedAt:   request.RequestedAt,
	}, nil
}
