package worker

import (
	"context"
	"log"

	"github.com/diogomassis/rinha-2025/internal/models"
)

type RinhaJobFunc func(ctx context.Context, data models.RinhaPendingPayment) error

func ExampleLoggingJob(ctx context.Context, data models.RinhaPendingPayment) error {
	log.Printf("[worker] received pending payment with Correlation Id: %s", data.CorrelationId)
	return nil
}
