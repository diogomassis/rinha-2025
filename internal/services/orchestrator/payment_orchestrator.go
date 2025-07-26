package orchestrator

import (
	"context"
	"errors"
	"sort"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/health"
	"github.com/diogomassis/rinha-2025/internal/services/processor"
)

type candidateProcessor struct {
	processor processor.PaymentProcessor
	health    processor.HealthStatus
}

type RinhaPaymentOrchestrator struct {
	processors    []processor.PaymentProcessor
	healthMonitor *health.RinhaMonitor
}

func NewRinhaPaymentOrchestrator(healthMonitor *health.RinhaMonitor, processors ...processor.PaymentProcessor) *RinhaPaymentOrchestrator {
	return &RinhaPaymentOrchestrator{
		processors:    processors,
		healthMonitor: healthMonitor,
	}
}

func (o *RinhaPaymentOrchestrator) ExecutePayment(ctx context.Context, payment models.RinhaPendingPayment) (*models.CompletedPayment, error) {
	candidates := make([]candidateProcessor, 0, len(o.processors))
	for _, p := range o.processors {
		status, found := o.healthMonitor.GetStatus(p.GetName())
		if found && !status.Failing {
			candidates = append(candidates, candidateProcessor{processor: p, health: status})
		}
	}

	if len(candidates) == 0 {
		return nil, errors.New("no healthy payment processors available")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].health.MinResponseTime < candidates[j].health.MinResponseTime
	})

	for _, candidate := range candidates {
		proc := candidate.processor

		timeUtc, err := proc.ProcessPayment(ctx, &payment)
		if err == nil {
			return &models.CompletedPayment{
				CorrelationID: payment.CorrelationId,
				Amount:        payment.Amount,
				Type:          proc.GetName(),
				ProcessedAt:   timeUtc,
			}, nil
		}
		if errors.Is(err, processor.ErrPaymentDefinitive) {
			return nil, err
		}
	}
	return nil, errors.New("all healthy and sorted payment processors failed")
}
