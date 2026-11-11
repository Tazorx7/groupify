// Package domain holds the shared vocabulary of the application: typed
// errors that map to HTTP status codes, input normalisation rules and
// pagination primitives. It must not import net/http or pgx.
package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a business error carrying a stable machine-readable code, a message
// safe to show a user, and optional per-field validation messages.
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	status  int
	wrap    error
}

func (e *Error) Error() string {
	if e.wrap != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.wrap)
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.wrap }

// Is makes errors.Is match any error carrying the same code, so a sentinel with
// extra field messages added via WithField still compares equal to the sentinel.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.status == t.status
}

// Status is the HTTP status code this error maps to.
func (e *Error) Status() int {
	if e.status == 0 {
		return http.StatusBadRequest
	}
	return e.status
}

// WithField returns a copy carrying an additional field-level message. The
// receiver is never mutated, so package-level sentinels stay safe to share.
func (e *Error) WithField(field, message string) *Error {
	clone := *e
	clone.Fields = make(map[string]string, len(e.Fields)+1)
	for k, v := range e.Fields {
		clone.Fields[k] = v
	}
	clone.Fields[field] = message
	return &clone
}

// WithFields returns a copy carrying field-level messages.
func (e *Error) WithFields(fields map[string]string) *Error {
	if len(fields) == 0 {
		return e
	}
	clone := *e
	clone.Fields = make(map[string]string, len(e.Fields)+len(fields))
	for k, v := range e.Fields {
		clone.Fields[k] = v
	}
	for k, v := range fields {
		clone.Fields[k] = v
	}
	return &clone
}

func newError(status int, code, message string) *Error {
	return &Error{status: status, Code: code, Message: message}
}

// Sentinel business errors. Compare with errors.Is.
var (
	ErrRegistrationClosed = newError(http.StatusConflict, "registration_closed",
		"Registration is currently closed.")
	ErrTopicFull = newError(http.StatusConflict, "topic_full",
		"This topic is full. Please choose another topic.")
	ErrTopicInactive = newError(http.StatusConflict, "topic_inactive",
		"This topic is not open for registration.")
	ErrTopicNotFound = newError(http.StatusNotFound, "topic_not_found",
		"That topic does not exist.")
	ErrIndexNumberTaken = newError(http.StatusConflict, "index_number_already_registered",
		"This index number is already registered.")
	ErrCapacityBelowMembers = newError(http.StatusConflict, "capacity_below_members",
		"Capacity cannot be lower than the number of students already registered.")
	ErrTopicNameTaken = newError(http.StatusConflict, "topic_name_taken",
		"A topic with that name already exists.")
	ErrRegistrationNotFound = newError(http.StatusNotFound, "registration_not_found",
		"That registration does not exist.")
	ErrAdminNotFound = newError(http.StatusNotFound, "admin_not_found",
		"Admin account not found.")
	ErrInvalidCredentials = newError(http.StatusUnauthorized, "invalid_credentials",
		"Email or password is incorrect.")
	ErrAuthenticationRequired = newError(http.StatusUnauthorized, "authentication_required",
		"Admin authentication is required.")
	ErrCSRF = newError(http.StatusForbidden, "csrf_check_failed",
		"Request could not be verified. Refresh the page and try again.")
	ErrForbidden = newError(http.StatusForbidden, "forbidden",
		"You do not have permission to perform this action.")
	ErrNotFound = newError(http.StatusNotFound, "not_found", "Not found.")
	ErrConflict = newError(http.StatusConflict, "conflict",
		"The request conflicts with the current state.")
	ErrIdempotencyReuse = newError(http.StatusUnprocessableEntity, "idempotency_key_reuse",
		"This idempotency key was already used for a different request.")
	ErrIdempotencyInProgress = newError(http.StatusConflict, "idempotency_key_in_progress",
		"An identical request is still being processed. Please retry shortly.")
	ErrRateLimited = newError(http.StatusTooManyRequests, "rate_limited",
		"Too many requests. Please wait a moment and try again.")
	ErrPayloadTooLarge = newError(http.StatusRequestEntityTooLarge, "payload_too_large",
		"Request body is too large.")
	ErrInvalidJSON = newError(http.StatusBadRequest, "invalid_json",
		"Request body must be valid JSON.")
	ErrMethodNotAllowed = newError(http.StatusMethodNotAllowed, "method_not_allowed",
		"That method is not allowed on this resource.")
	ErrUnavailable = newError(http.StatusServiceUnavailable, "unavailable",
		"The service is temporarily unavailable.")
	ErrInternal = newError(http.StatusInternalServerError, "internal_error",
		"Something went wrong. Please try again.")
)

// ValidationError builds a 422 error with field-level messages.
func ValidationError(message string, fields map[string]string) *Error {
	e := newError(http.StatusUnprocessableEntity, "validation_failed", message)
	return e.WithFields(fields)
}

// AsError extracts a *Error from err, or nil.
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
