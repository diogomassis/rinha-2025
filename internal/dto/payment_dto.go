package dto

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

type PaymentSummaryResponse struct {
	Default  PaymentSummaryItemResponse `json:"default"`
	Fallback PaymentSummaryItemResponse `json:"fallback"`
}

type PaymentSummaryItemResponse struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}
