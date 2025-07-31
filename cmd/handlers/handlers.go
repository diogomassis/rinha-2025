package handlers

import (
	"time"

	"github.com/diogomassis/rinha-2025/internal/dto"
	"github.com/diogomassis/rinha-2025/internal/persistence"
	chooserchecker "github.com/diogomassis/rinha-2025/internal/services/chooser-checker"
	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
	"github.com/diogomassis/rinha-2025/internal/services/worker"
	"github.com/gofiber/fiber/v2"
)

var (
	Chooser       *chooserchecker.ChooserService
	HealthChecker *healthchecker.HealthChecker
	Persistence   *persistence.PaymentPersistenceService
	Worker        *worker.Worker
)

func HandlePostPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	paymentReq := &paymentprocessor.PaymentRequest{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		RequestedAt:   time.Now().UTC(),
	}
	err := Worker.AddPaymentJob(paymentReq)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func HandleGetSummary(c *fiber.Ctx) error {
	var res dto.PaymentSummaryResponse
	return c.JSON(res)
}
