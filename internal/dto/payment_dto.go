package dto

type PaymentSummaryResponse struct {
	Default  PaymentSummaryItemResponse `json:"default"`
	Fallback PaymentSummaryItemResponse `json:"fallback"`
}

type PaymentSummaryItemResponse struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}
