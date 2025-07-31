package handlers

import (
	"github.com/diogomassis/rinha-2025/internal/dto"
	"github.com/diogomassis/rinha-2025/internal/persistence"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	"github.com/gofiber/fiber/v2"
)

var (
	Persistence *persistence.PaymentPersistenceService
	Monitor     *healthchecker.HealthChecker
	Chooser     *chooserchecker.ChooserService
)

func HandlePostPayment(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusAccepted)
}

func HandleGetSummary(c *fiber.Ctx) error {
	var res dto.PaymentSummaryResponse
	return c.JSON(res)
}
