package models

type PaymentEntity struct {
	CorrelationId string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

func NewPaymentEntity(correlationId string, amount float64) *PaymentEntity {
	return &PaymentEntity{
		CorrelationId: correlationId,
		Amount:        amount,
	}
}
