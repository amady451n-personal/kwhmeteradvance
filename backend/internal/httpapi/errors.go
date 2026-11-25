package httpapi

import "meteran/internal/store"

// Error codes from docs/api/openapi.yaml. Keep in sync.
const (
	CodeValidationError      = "validation_error"
	CodeConfirmationRequired = "confirmation_required"
	CodeUnauthorized         = "unauthorized"
	CodePinInvalid           = "pin_invalid"
	CodeRateLimited          = "rate_limited"
	CodeNotFound             = "not_found"
	CodeConflict             = "conflict"
	CodeReadingLocked        = "reading_locked"
	CodeTokenLocked          = "token_locked"
	CodeTokenNotDeletable    = "token_not_deletable"
	CodeImportNotAllowed     = "import_not_allowed"
	CodeUnsupportedMediaType = "unsupported_media_type"
	CodePayloadTooLarge      = "payload_too_large"
	CodeInternal             = "internal_error"
)

// Confirmation detail codes.
const (
	ConfirmIncreasedWithoutTopup = store.ConfirmIncreasedWithoutTopup
	ConfirmDuplicateTimestamp    = store.ConfirmDuplicateTimestamp
	ConfirmDenominationOutOfTol  = store.ConfirmDenominationOutOfTol
)