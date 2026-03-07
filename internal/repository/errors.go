package repository

import "errors"

// Sentinel errors for repository operations.
var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrNotFound            = errors.New("record not found")
	ErrDuplicateKey        = errors.New("duplicate idempotency key")
)
