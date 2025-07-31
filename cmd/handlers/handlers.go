package handlers

import (
	"github.com/diogomassis/rinha-2025/internal/dto"
	"github.com/gofiber/fiber/v2"
)

func HandlePostPayment(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusAccepted)
}

func HandleGetSummary(c *fiber.Ctx) error {
	var res dto.PaymentSummaryResponse
	return c.JSON(res)
}
