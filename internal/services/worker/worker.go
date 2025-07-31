package worker

import (
	"github.com/diogomassis/rinha-2025/internal/env"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
)

type Worker struct {
	processors map[string]*paymentprocessor.PaymentProcessor
}

func New() *Worker {
	return &Worker{
		processors: map[string]*paymentprocessor.PaymentProcessor{
			"d": paymentprocessor.New("d", env.Env.ProcessorDefaultUrl),
			"f": paymentprocessor.New("f", env.Env.ProcessorFallbackUrl),
		},
	}
}
