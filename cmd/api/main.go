package main

import (
	"context"
	"log"

	"github.com/diogomassis/rinha-2025/internal/env"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

type SummaryResponse struct {
	Default  Summary `json:"default"`
	Fallback Summary `json:"fallback"`
}

type Summary struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

var db *pgxpool.Pool

func main() {
	env.Load()

	var err error
	ctx := context.Background()
	db, err = pgxpool.New(ctx, env.Env.DbUrl)
	if err != nil {
		log.Fatal("failed to connect to DB:", err)
	}
	defer db.Close()

	app := fiber.New()

	app.Post("/payments", handlePostPayment)
	app.Get("/payments-summary", handleGetSummary)

	log.Fatal(app.Listen(":" + env.Env.Port))
}

func handlePostPayment(c *fiber.Ctx) error {
	var req PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	if req.CorrelationID == "" || req.Amount <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid fields")
	}

	_, err := db.Exec(c.Context(), `
		INSERT INTO payments (correlation_id, amount, processor, created_at)
		VALUES ($1, $2, $3, now())`,
		req.CorrelationID, req.Amount, "default")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "DB error")
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func handleGetSummary(c *fiber.Ctx) error {
	ctx := c.Context()
	var res SummaryResponse

	err := db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM payments
		WHERE processor = 'default'`,
	).Scan(&res.Default.TotalRequests, &res.Default.TotalAmount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "DB error")
	}

	err = db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM payments
		WHERE processor = 'fallback'`,
	).Scan(&res.Fallback.TotalRequests, &res.Fallback.TotalAmount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "DB error")
	}

	return c.JSON(res)
}
