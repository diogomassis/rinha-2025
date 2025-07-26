package models

import "time"

type RinhaPendingPayment struct {
	CorrelationId string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

func NewRinhaPendingPayment(correlationId string, amount float64) *RinhaPendingPayment {
	return &RinhaPendingPayment{
		CorrelationId: correlationId,
		Amount:        amount,
	}
}

func (r *RinhaPendingPayment) SetRequestedAt(requestedAt time.Time) {
	r.RequestedAt = requestedAt.UTC()
}

type PaymentSummary struct {
	Default  PaymentSummaryItem `json:"default"`
	Fallback PaymentSummaryItem `json:"fallback"`
}

func NewPaymentSummary(defaultItem, fallbackItem PaymentSummaryItem) *PaymentSummary {
	return &PaymentSummary{
		Default:  defaultItem,
		Fallback: fallbackItem,
	}
}

type PaymentSummaryItem struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

func NewPaymentSummaryItem(totalRequests int64, totalAmount float64) *PaymentSummaryItem {
	return &PaymentSummaryItem{
		TotalRequests: totalRequests,
		TotalAmount:   totalAmount,
	}
}

type CompletedPayment struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	Type          string    `json:"type"`
	ProcessedAt   time.Time `json:"processedAt"`
}

func NewCompletedPayment(correlationID string, amount float64, paymentType string, processedAt time.Time) *CompletedPayment {
	return &CompletedPayment{
		CorrelationID: correlationID,
		Amount:        amount,
		Type:          paymentType,
		ProcessedAt:   processedAt,
	}
}

func (c *CompletedPayment) SetType(paymentType string) {
	c.Type = paymentType
}
