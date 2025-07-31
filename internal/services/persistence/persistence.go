package persistence

import (
	"context"

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

func (pps *PaymentPersistenceService) SavePayment(payment *paymentprocessor.PaymentResponse) (int64, error) {
	query := `
		INSERT INTO payments (correlation_id, amount, processor, requested_at) VALUES ($1, $2, $3, $4)
	`
	arguments := []any{
		payment.CorrelationID,
		payment.Amount,
		payment.Processor,
		payment.RequestedAt,
	}
	command, err := pps.db.Exec(context.Background(), query, arguments...)
	return command.RowsAffected(), err
}
