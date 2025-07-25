package orchestrator

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

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
		} else {
			log.Printf("[orchestrator] Skipping processor %s due to unhealthy status (Found: %t, Status: %+v)", p.GetName(), found, status)
		}
	}

	if len(candidates) == 0 {
		log.Printf("[orchestrator] CRITICAL: No healthy payment processors available.")
		return nil, errors.New("no healthy payment processors available")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].health.MinResponseTime < candidates[j].health.MinResponseTime
	})
	log.Printf("[orchestrator] Determined processor priority: %v", getCandidateNames(candidates))

	for _, candidate := range candidates {
		proc := candidate.processor
		log.Printf("[orchestrator] Attempting payment with dynamically chosen processor: %s", proc.GetName())

		err := proc.ProcessPayment(ctx, &payment)
		if err == nil {
			log.Printf("[orchestrator] Payment successful with processor: %s", proc.GetName())
			return &models.CompletedPayment{
				CorrelationID: payment.CorrelationId,
				Amount:        payment.Amount,
				Type:          proc.GetName(),
				ProcessedAt:   time.Now(),
			}, nil
		}
		if errors.Is(err, processor.ErrPaymentDefinitive) {
			log.Printf("[orchestrator] DEFINITIVE error with %s: %v. Halting attempts.", proc.GetName(), err)
			return nil, err
		}
		log.Printf("[orchestrator] TRANSIENT error with %s: %v. Trying next processor...", proc.GetName(), err)
	}
	log.Printf("[orchestrator] CRITICAL: All healthy payment processors failed for CorrelationId: %s", payment.CorrelationId)
	return nil, errors.New("all healthy and sorted payment processors failed")
}

func getCandidateNames(candidates []candidateProcessor) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.processor.GetName()
	}
	return names
}
