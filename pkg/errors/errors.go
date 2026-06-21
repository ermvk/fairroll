package errors

import "fmt"

type ErrorType string

const (
	ErrorTypeInvalidCredentials  ErrorType = "INVALID_CREDENTIALS"
	ErrorTypeConflict            ErrorType = "CONFLICT"
	ErrorTypeNotFound            ErrorType = "NOT_FOUND"
	ErrorTypeUnauthorized        ErrorType = "UNAUTHORIZED"
	ErrorTypeForbidden           ErrorType = "FORBIDDEN"
	ErrorTypeBadRequest          ErrorType = "BAD_REQUEST"
	ErrorTypeDatabase            ErrorType = "DATABASE_ERROR"
	ErrorTypeInternal            ErrorType = "INTERNAL_ERROR"
	ErrorTypeValidation          ErrorType = "VALIDATION_ERROR"
	ErrorTypeRiskBlocked         ErrorType = "RISK_BLOCKED"
	ErrorTypeInsufficientBalance ErrorType = "INSUFFICIENT_BALANCE"
)

type AppError struct {
	Type       ErrorType
	Message    string
	StatusCode int
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s : %s", e.Type, e.Message)
}

func NewInvalidCredentialsError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeInvalidCredentials,
		Message:    message,
		StatusCode: 401,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeConflict,
		Message:    message,
		StatusCode: 409,
	}
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: 404,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Message:    message,
		StatusCode: 401,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeForbidden,
		Message:    message,
		StatusCode: 403,
	}
}

func NewBadRequestError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeBadRequest,
		Message:    message,
		StatusCode: 400,
	}
}

func NewDatabaseError(err error) *AppError {
	return &AppError{
		Type:       ErrorTypeDatabase,
		Message:    "Database operation failed",
		StatusCode: 500,
		Err:        err,
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    "Internal server error",
		StatusCode: 500,
		Err:        err,
	}
}

func NewValidationError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		StatusCode: 400,
	}
}

func NewRiskBlockedError(reason string) *AppError {
	return &AppError{
		Type:       ErrorTypeRiskBlocked,
		Message:    fmt.Sprintf("Operation blocked by risk engine: %s", reason),
		StatusCode: 403,
	}
}

func NewInsufficientBalanceError() *AppError {
	return &AppError{
		Type:       ErrorTypeInsufficientBalance,
		Message:    "Insufficient balance for this operation",
		StatusCode: 400,
	}
}

// Helper fnx
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

func AsAppError(err error) (*AppError, bool) {
	appError, ok := err.(*AppError)
	return appError, ok
}
