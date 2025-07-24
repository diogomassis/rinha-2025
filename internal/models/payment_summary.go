package models

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
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

func NewPaymentSummaryItem(totalRequests int, totalAmount float64) *PaymentSummaryItem {
	return &PaymentSummaryItem{
		TotalRequests: totalRequests,
		TotalAmount:   totalAmount,
	}
}
