package store

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrReadingLocked     = errors.New("reading linked to locked token")
	ErrTokenLocked       = errors.New("token locked")
	ErrTokenNotDeletable = errors.New("token not deletable")
	ErrImportNotAllowed  = errors.New("import not allowed: database not empty")
)

// Confirmation codes the client must echo back to override a soft violation.
const (
	ConfirmIncreasedWithoutTopup = "increased_without_topup"
	ConfirmDuplicateTimestamp    = "duplicate_timestamp"
	ConfirmDenominationOutOfTol  = "denomination_out_of_tolerance"
)

// Detail is a machine-readable reason for a rejected request.
type Detail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrConfirmation carries soft violations the user must approve before the
// write is retried with confirm codes.
type ErrConfirmation struct{ Details []Detail }

func (e ErrConfirmation) Error() string { return "confirmation required" }

// ErrValidation carries hard violations (422).
type ErrValidation struct{ Details []Detail }

func (e ErrValidation) Error() string { return "validation error" }

func validation(field, code, msg string) error {
	return ErrValidation{Details: []Detail{{Field: field, Code: code, Message: msg}}}
}