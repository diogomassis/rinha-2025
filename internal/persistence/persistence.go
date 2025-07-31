package persistence

import (
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentPersistenceService struct {
	db *pgxpool.Pool
}

func NewPaymentPersistenceService(db *pgxpool.Pool) *PaymentPersistenceService {
	return &PaymentPersistenceService{
		db: db,
	}
}

func (pps *PaymentPersistenceService) SavePayment(payment *paymentprocessor.PaymentResponse) error {
	return nil
}
