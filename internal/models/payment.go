package models

type RinhaPendingPayment struct {
	CorrelationId string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

func NewRinhaPendingPayment(correlationId string, amount float64) *RinhaPendingPayment {
	return &RinhaPendingPayment{
		CorrelationId: correlationId,
		Amount:        amount,
	}
}
