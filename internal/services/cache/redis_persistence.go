package cache

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/diogomassis/rinha-2025/internal/models"
	"github.com/redis/go-redis/v9"
)

type RinhaRedisPersistenceService struct {
	client *redis.Client
}

func NewRinhaRedisPersistenceService(client *redis.Client) *RinhaRedisPersistenceService {
	return &RinhaRedisPersistenceService{
		client: client,
	}
}

func (r *RinhaRedisPersistenceService) Add(ctx context.Context, p models.CompletedPayment) error {
	key := fmt.Sprintf("payments:series:%s", p.Type)
	timestamp := p.ProcessedAt.UnixMilli()
	member := fmt.Sprintf("%s:%.2f", p.CorrelationID, p.Amount)

	_, err := r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(timestamp),
		Member: member,
	}).Result()
	return err
}

func (r *RinhaRedisPersistenceService) Get(ctx context.Context, from, to time.Time) (*models.PaymentSummary, error) {
	summary := &models.PaymentSummary{}

	var err error
	summary.Default, err = r.getSummaryForType(ctx, "default", from, to)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar resumo 'default': %w", err)
	}

	summary.Fallback, err = r.getSummaryForType(ctx, "fallback", from, to)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar resumo 'fallback': %w", err)
	}
	return summary, nil
}

func (r *RinhaRedisPersistenceService) getSummaryForType(ctx context.Context, paymentType string, from, to time.Time) (models.PaymentSummaryItem, error) {
	key := fmt.Sprintf("payments:series:%s", paymentType)
	min := "-inf"
	max := "+inf"
	if !from.IsZero() {
		min = strconv.FormatInt(from.UnixMilli(), 10)
	}
	if !to.IsZero() {
		max = strconv.FormatInt(to.UnixMilli(), 10)
	}

	members, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
	if err != nil {
		return models.PaymentSummaryItem{}, err
	}

	var totalAmount float64
	totalRequests := int64(len(members))
	for _, member := range members {
		parts := strings.Split(member, ":")
		if len(parts) < 2 {
			continue
		}
		amount, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			log.Printf("[persistence] ERROR: Could not parse amount from member '%s'. Error: %v", member, err)
			continue
		}
		totalAmount += amount
	}
	return models.PaymentSummaryItem{
		TotalRequests: totalRequests,
		TotalAmount:   totalAmount,
	}, nil
}
