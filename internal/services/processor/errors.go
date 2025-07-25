package processor

import "errors"

var (
	ErrServiceUnavailable = errors.New("payment processor service is unavailable")
	ErrPaymentDefinitive  = errors.New("definitive payment processing error")
)
