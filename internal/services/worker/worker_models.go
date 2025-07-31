package worker

import paymentprocessor "github.com/diogomassis/rinha-2025/internal/services/payment-processor"

type Job struct {
	requestPayment *paymentprocessor.PaymentRequest
}
