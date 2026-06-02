package types

import (
	stderrors "errors"

	"cosmossdk.io/errors"
)

var (
	ErrInvalidRequest = errors.Register(ModuleName, 0, "invalid request")
	ErrVaultNotFound  = errors.Register(ModuleName, 1, "vault not found")
)

// Refund-classification sentinel errors raised inside this module and wrapped at their source, so
// getRefundReason can classify them with errors.Is instead of matching free-form error text.
var (
	ErrNavNotFound      = stderrors.New("nav not found")
	ErrReconcileFailure = stderrors.New("reconcile failure")
)

// CriticalError wraps an error that represents a critical, unrecoverable failure
// requiring automatic vault pausing. It includes a stable, hard-coded Reason
// string that is persisted in state, decoupled from SDK or underlying error text.
type CriticalError struct {
	// Reason is a stable, hard-coded description of why the vault must be paused.
	Reason string
	// Err is the underlying error, which may include deeper SDK or keeper details.
	Err error
}

// Error implements the error interface by returning the underlying error message.
func (e *CriticalError) Error() string { return e.Err.Error() }

// Unwrap allows errors.Unwrap and errors.Is/As to inspect the underlying error.
func (e *CriticalError) Unwrap() error { return e.Err }

// CriticalErr constructs a new CriticalError with the given reason string and underlying error.
// Callers should use this helper when a failure is unrecoverable and the vault should be auto-paused.
func CriticalErr(reason string, err error) error {
	return &CriticalError{Reason: reason, Err: err}
}
