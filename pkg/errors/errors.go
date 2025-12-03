package errors

import "fmt"

// Error types for the session service
var (
	ErrInvalidCredentials = &ServiceError{
		Code:    "INVALID_CREDENTIALS",
		Message: "Invalid client credentials",
		Status:  401,
	}

	ErrInvalidToken = &ServiceError{
		Code:    "INVALID_TOKEN",
		Message: "Invalid or expired token",
		Status:  401,
	}

	ErrTokenRevoked = &ServiceError{
		Code:    "TOKEN_REVOKED",
		Message: "Token has been revoked",
		Status:  401,
	}

	ErrRateLimitExceeded = &ServiceError{
		Code:    "RATE_LIMIT_EXCEEDED",
		Message: "Rate limit exceeded",
		Status:  429,
	}

	ErrInvalidGrant = &ServiceError{
		Code:    "INVALID_GRANT",
		Message: "Invalid grant type",
		Status:  400,
	}

	// ErrInvalidRequest is used for syntactically invalid requests (missing or
	// malformed parameters) where a 400 response is appropriate.
	ErrInvalidRequest = &ServiceError{
		Code:    "INVALID_REQUEST",
		Message: "Invalid request",
		Status:  400,
	}

	ErrInvalidRefreshToken = &ServiceError{
		Code:    "INVALID_REFRESH_TOKEN",
		Message: "Invalid or expired refresh token",
		Status:  401,
	}

	ErrInternalServer = &ServiceError{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Internal server error",
		Status:  500,
	}
)

// ServiceError represents a service-level error
type ServiceError struct {
	Code    string
	Message string
	Status  int
	Err     error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// Wrap wraps an error with a ServiceError
func Wrap(err error, serviceErr *ServiceError) *ServiceError {
	return &ServiceError{
		Code:    serviceErr.Code,
		Message: serviceErr.Message,
		Status:  serviceErr.Status,
		Err:     err,
	}
}

