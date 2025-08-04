package handlers

import (
	"errors"

	"github.com/diogomassis/rinha-2025/internal/dto"
	paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"
	"github.com/diogomassis/rinha-2025/internal/services/persistence"
	"github.com/diogomassis/rinha-2025/internal/services/worker"
	"github.com/gofiber/fiber/v2"
)

var (
	Persistence *persistence.PaymentPersistenceService
	Worker      *worker.Worker
)

func HandlePostPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	paymentReq := &paymentprocessor.PaymentRequest{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
	}

	err := Worker.AddPaymentJob(paymentReq)
	if err != nil {
		if errors.Is(err, worker.ErrQueueFull) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service unavailable, please try again later"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error while processing payment"})
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func HandleGetSummary(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	summary, err := Persistence.GetPaymentSummary(from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve payment summary -> " + err.Error()})
	}
	var res dto.PaymentSummaryResponse
	res.Default = dto.PaymentSummaryItemResponse{
		TotalRequests: summary.Default.TotalRequests,
		TotalAmount:   summary.Default.TotalAmount,
	}
	res.Fallback = dto.PaymentSummaryItemResponse{
		TotalRequests: summary.Fallback.TotalRequests,
		TotalAmount:   summary.Fallback.TotalAmount,
	}
	return c.JSON(res)
}
