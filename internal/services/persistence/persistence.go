package persistence

import (
	"context"
	"time"

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
		INSERT INTO payments (correlation_id, amount, processor, requested_at) 
		VALUES ($1, $2, $3, $4)
	`
	command, err := pps.db.Exec(context.Background(), query,
		payment.CorrelationID,
		payment.Amount,
		payment.Processor,
		payment.RequestedAt,
	)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (pps *PaymentPersistenceService) GetPaymentSummary(from, to time.Time) (paymentprocessor.PaymentSummaryResponse, error) {
	query := `SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM payments WHERE processor = $1 AND requested_at BETWEEN $2 AND $3`
	defaultSummary := paymentprocessor.PaymentSummaryItemResponse{}
	fallbackSummary := paymentprocessor.PaymentSummaryItemResponse{}
	err := pps.db.QueryRow(context.Background(), query, "d", from, to).Scan(&defaultSummary.TotalRequests, &defaultSummary.TotalAmount)
	if err != nil {
		return paymentprocessor.PaymentSummaryResponse{}, err
	}
	err = pps.db.QueryRow(context.Background(), query, "f", from, to).Scan(&fallbackSummary.TotalRequests, &fallbackSummary.TotalAmount)
	if err != nil {
		return paymentprocessor.PaymentSummaryResponse{}, err
	}
	return paymentprocessor.PaymentSummaryResponse{
		Default:  defaultSummary,
		Fallback: fallbackSummary,
	}, nil
}
