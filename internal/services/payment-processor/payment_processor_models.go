package paymentprocessor

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt"`
}

type PaymentResponse struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	Processor     string  `json:"processor"`
	RequestedAt   string  `json:"requestedAt"`
}

type PaymentSummaryItemResponse struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type PaymentSummaryResponse struct {
	Default  PaymentSummaryItemResponse `json:"default"`
	Fallback PaymentSummaryItemResponse `json:"fallback"`
}
