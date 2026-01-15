package service

import (
	"errors"
	"fmt"
)

var (
	// ErrOrderNotFound is returned when an order doesn't exist
	ErrOrderNotFound = errors.New("order not found")

	// ErrNoItems is returned when trying to create an order with no items
	ErrNoItems = errors.New("order must have at least one item")

	// ErrMissingEmail is returned when customer email is missing
	ErrMissingEmail = errors.New("customer email is required")

	// ErrPaymentServiceUnavailable is returned when payment service is down
	ErrPaymentServiceUnavailable = errors.New("payment service unavailable")
)

// PaymentDeclinedError is returned when payment is declined
type PaymentDeclinedError struct {
	Code    string
	Message string
}

func (e *PaymentDeclinedError) Error() string {
	return fmt.Sprintf("payment declined: %s - %s", e.Code, e.Message)
}

// IsPaymentDeclined checks if an error is a payment declined error
func IsPaymentDeclined(err error) bool {
	_, ok := err.(*PaymentDeclinedError)
	return ok
}
