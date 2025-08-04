package handlers

import (
	"errors"
	"strings"
	"time"

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
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "both 'from' and 'to' query parameters are required"})
	}
	if !strings.HasSuffix(fromStr, "Z") {
		fromStr = fromStr + "Z"
	}
	if !strings.HasSuffix(toStr, "Z") {
		toStr = toStr + "Z"
	}

	layout := time.RFC3339Nano
	fromTime, err := time.Parse(layout, fromStr)
	if err != nil {
		layout = "2006-01-02T15:04:05.000Z"
		fromTime, err = time.Parse(layout, fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date format"})
		}
	}

	toTime, err := time.Parse(layout, toStr)
	if err != nil {
		layout = "2006-01-02T15:04:05.000Z"
		toTime, err = time.Parse(layout, toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date format"})
		}
	}

	summary, err := Persistence.GetPaymentSummary(fromTime, toTime)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch payment summary -> " + err.Error()})
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
