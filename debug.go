package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type DebugHandler struct {
	redisClient *redis.Client
}

type PaymentDebugInfo struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	Processor     string    `json:"processor"`
	ProcessedAt   time.Time `json:"processedAt"`
	StoredData    string    `json:"storedData"`
}

func NewDebugHandler(redisClient *redis.Client) *DebugHandler {
	return &DebugHandler{
		redisClient: redisClient,
	}
}

func (dh *DebugHandler) DebugPayment(w http.ResponseWriter, r *http.Request) {
	correlationId := r.URL.Query().Get("correlationId")
	if correlationId == "" {
		http.Error(w, "correlationId parameter required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	recordKey := fmt.Sprintf("payment:%s", correlationId)

	recordJSON, err := dh.redisClient.Get(ctx, recordKey).Result()
	if err != nil {
		if err == redis.Nil {
			http.Error(w, "Payment not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var record map[string]interface{}
	if err := json.Unmarshal([]byte(recordJSON), &record); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	processedAt, _ := time.Parse(time.RFC3339, record["processedAt"].(string))

	debugInfo := PaymentDebugInfo{
		CorrelationID: record["correlationId"].(string),
		Amount:        record["amount"].(float64),
		Processor:     record["processor"].(string),
		ProcessedAt:   processedAt,
		StoredData:    recordJSON,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

func (dh *DebugHandler) DebugSummary(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	summary := make(map[string]interface{})

	for _, processor := range []string{"default", "fallback"} {
		countKey := fmt.Sprintf("payments:%s:count", processor)
		count, _ := dh.redisClient.Get(ctx, countKey).Int64()

		amountKey := fmt.Sprintf("payments:%s:amount", processor)
		amount, _ := dh.redisClient.Get(ctx, amountKey).Float64()

		listKey := fmt.Sprintf("payments:list:%s", processor)
		listSize, _ := dh.redisClient.ZCard(ctx, listKey).Result()

		summary[processor] = map[string]interface{}{
			"aggregatedCount":  count,
			"aggregatedAmount": amount,
			"listSize":         listSize,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (dh *DebugHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	processor := r.URL.Query().Get("processor")
	if processor == "" {
		processor = "default"
	}

	listKey := fmt.Sprintf("payments:list:%s", processor)
	correlationIds, err := dh.redisClient.ZRevRange(ctx, listKey, 0, 9).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var payments []PaymentDebugInfo
	for _, correlationId := range correlationIds {
		recordKey := fmt.Sprintf("payment:%s", correlationId)
		recordJSON, err := dh.redisClient.Get(ctx, recordKey).Result()
		if err != nil {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(recordJSON), &record); err != nil {
			continue
		}

		processedAt, _ := time.Parse(time.RFC3339, record["processedAt"].(string))

		payments = append(payments, PaymentDebugInfo{
			CorrelationID: record["correlationId"].(string),
			Amount:        record["amount"].(float64),
			Processor:     record["processor"].(string),
			ProcessedAt:   processedAt,
			StoredData:    recordJSON,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func RegisterDebugRoutes(redisClient *redis.Client) {
	debugHandler := NewDebugHandler(redisClient)

	http.HandleFunc("/debug/payment", debugHandler.DebugPayment)
	http.HandleFunc("/debug/summary", debugHandler.DebugSummary)
	http.HandleFunc("/debug/payments", debugHandler.ListPayments)

	log.Println("Debug endpoints registered:")
	log.Println("  GET /debug/payment?correlationId=<id>")
	log.Println("  GET /debug/summary")
	log.Println("  GET /debug/payments?processor=<default|fallback>")
}
