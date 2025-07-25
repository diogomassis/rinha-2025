package orchestrator

import "github.com/diogomassis/rinha-2025/internal/services/processor"

type PaymentOrchestrator struct {
	processors []processor.PaymentProcessor
}

func NewPaymentOrchestrator(processors ...processor.PaymentProcessor) *PaymentOrchestrator {
	return &PaymentOrchestrator{
		processors: processors,
	}
}
