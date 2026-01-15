package service

import "errors"

var (
	// ErrTransactionNotFound is returned when a transaction doesn't exist
	ErrTransactionNotFound = errors.New("transaction not found")
)
