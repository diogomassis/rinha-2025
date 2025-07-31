package main

import (
	"context"
	"log"

	"github.com/diogomassis/rinha-2025/internal/dto"
	"github.com/diogomassis/rinha-2025/internal/env"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	db      *pgxpool.Pool
	monitor *healthchecker.HealthChecker
	chooser *chooserchecker.ChooserService
)

func main() {
	env.Load()

	monitor = healthchecker.New()
	chooser = chooserchecker.New(monitor)
	monitor.Start()
	defer monitor.Stop()

	ctx := context.Background()
	db, err := pgxpool.New(ctx, env.Env.DbUrl)
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
	return c.SendStatus(fiber.StatusAccepted)
}

func handleGetSummary(c *fiber.Ctx) error {
	var res dto.PaymentSummaryResponse
	return c.JSON(res)
}
