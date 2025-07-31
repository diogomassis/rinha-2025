package main

import (
	"context"
	"log"
	"time"

	"github.com/diogomassis/rinha-2025/cmd/handlers"
	"github.com/diogomassis/rinha-2025/internal/env"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	"github.com/diogomassis/rinha-2025/internal/services/persistence"
	"github.com/diogomassis/rinha-2025/internal/services/worker"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IMPLEMENTAR O ENDPOINT DE RESUMO DE PAGAMENTOS

func main() {
	env.Load()

	healthChecker := healthchecker.New()
	healthChecker.Start()
	defer healthChecker.Stop()

	chooserChecker := chooserchecker.New(healthChecker)

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
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("Failed to connect to DB with optimized config:", err)
	}
	defer db.Close()

	paymentPersistenceService := persistence.NewPaymentPersistenceService(db)

	workerPool := worker.New(chooserChecker, paymentPersistenceService)
	workerPool.Start()
	defer workerPool.Stop()

	handlers.HealthChecker = healthChecker
	handlers.Chooser = chooserChecker
	handlers.Persistence = paymentPersistenceService

	app := fiber.New()
	app.Post("/payments", handlers.HandlePostPayment)
	app.Get("/payments-summary", handlers.HandleGetSummary)

	log.Fatal(app.Listen(":" + env.Env.Port))
}
