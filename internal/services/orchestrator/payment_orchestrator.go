package orchestrator

import (
	"context"
	"errors"
	"sort"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/diogomassis/rinha-2025/internal/services/health"
	"github.com/diogomassis/rinha-2025/internal/services/processor"
)

var (
	NoHealthyPaymentAvailable = errors.New("no healthy payment processors available")
	AllHealthyPaymentFailed   = errors.New("all healthy and sorted payment processors failed")
)

type candidateProcessor struct {
	processor processor.PaymentProcessor
	health    processor.HealthStatus
}

type RinhaPaymentOrchestrator struct {
	processors    []processor.PaymentProcessor
	healthMonitor *health.RinhaHealthCheckerMonitor
}

func NewRinhaPaymentOrchestrator(healthMonitor *health.RinhaHealthCheckerMonitor, processors ...processor.PaymentProcessor) *RinhaPaymentOrchestrator {
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
		return nil, NoHealthyPaymentAvailable
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].health.MinResponseTime < candidates[j].health.MinResponseTime
	})

	for _, candidate := range candidates {
		proc := candidate.processor

		completedPayment, err := proc.ProcessPayment(ctx, &payment)
		if errors.Is(err, processor.ErrPaymentDefinitive) {
			return nil, err
		}
		if err != nil {
			continue
		}
		completedPayment.SetType(proc.GetName())
		return completedPayment, nil
	}
	return nil, AllHealthyPaymentFailed
}
