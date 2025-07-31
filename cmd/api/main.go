package main

import (
	"context"
	"log"
	"time"

	"github.com/diogomassis/rinha-2025/cmd/handlers"
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

	config, err := pgxpool.ParseConfig(env.Env.DbUrl)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}
	config.MaxConns = 30
	config.MinConns = 10
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	ctx := context.Background()
	db, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("Failed to connect to DB with optimized config:", err)
	}
	defer db.Close()

	app := fiber.New()
	app.Post("/payments", handlers.HandlePostPayment)
	app.Get("/payments-summary", handlers.HandleGetSummary)

	log.Fatal(app.Listen(":" + env.Env.Port))
}
