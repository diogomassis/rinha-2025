package handlers

import (
	"github.com/diogomassis/rinha-2025/internal/dto"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	Db      *pgxpool.Pool
	Monitor *healthchecker.HealthChecker
	Chooser *chooserchecker.ChooserService
)

func HandlePostPayment(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusAccepted)
}

func HandleGetSummary(c *fiber.Ctx) error {
	var res dto.PaymentSummaryResponse
	return c.JSON(res)
}
