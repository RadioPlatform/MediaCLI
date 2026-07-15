package api

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

type ErrorCode string

const (
	ErrAuthFailed      ErrorCode = "authentication_failed"
	ErrAuthForbidden   ErrorCode = "forbidden"
	ErrRateLimited     ErrorCode = "rate_limited"
	ErrNotFound        ErrorCode = "not_found"
	ErrFolderNotFound  ErrorCode = "folder_not_found"
	ErrValidation      ErrorCode = "validation_error"
	ErrNetwork         ErrorCode = "network_error"
	ErrTLSCertificate  ErrorCode = "tls_certificate_error"
	ErrInvalidResponse ErrorCode = "invalid_response"
	ErrUploadFailed    ErrorCode = "upload_failed"
	ErrUnknown         ErrorCode = "unknown"
)

type APIError struct {
	Code       ErrorCode
	HTTPStatus int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func (e *APIError) FriendlyMessage() string {
	switch e.Code {
	case ErrAuthFailed:
		return "The CLI API key is invalid, expired, or has been revoked.\n\nGenerate a new key in Account Settings \u2192 CLI API keys."
	case ErrAuthForbidden:
		return "The CLI API key cannot access the selected station or media resource."
	case ErrRateLimited:
		return "The server enforces a 60 requests-per-minute limit. Retrying automatically\u2026"
	case ErrNotFound:
		return "The requested resource was not found."
	case ErrFolderNotFound:
		return "The specified media folder was not found."
	case ErrNetwork:
		return "Could not connect to the Radio Platform server. Check your internet connection and try again."
	case ErrTLSCertificate:
		return "Secure HTTPS connection to the Radio Platform server failed. Its TLS certificate may be missing, untrusted, expired, or issued for a different hostname. The connection was refused."
	case ErrInvalidResponse:
		return "Received an unexpected response from the Radio Platform server."
	case ErrValidation:
		return e.Message
	case ErrUploadFailed:
		return e.Message
	default:
		if e.Message != "" {
			return e.Message
		}
		return "An unexpected error occurred."
	}
}

func NewAPIError(code ErrorCode, status int, msg string, err error) *APIError {
	return &APIError{
		Code:       code,
		HTTPStatus: status,
		Message:    msg,
		Err:        err,
	}
}

func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

func AsAPIError(err error) *APIError {
	var ae *APIError
	if err != nil && errors.As(err, &ae) {
		return ae
	}
	return nil
}

func isTLSCertificateError(err error) bool {
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostnameError) || errors.As(err, &invalidCertificate) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "tls: failed to verify certificate") ||
		strings.Contains(message, "x509:") ||
		strings.Contains(message, "certificate signed by unknown authority") ||
		strings.Contains(message, "server gave http response to https client") ||
		strings.Contains(message, "first record does not look like a tls handshake")
}
