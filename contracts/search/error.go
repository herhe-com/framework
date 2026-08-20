package search

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCause describes an engine error cause.
type ErrorCause struct {
	Type      string      `json:"type,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Index     string      `json:"index,omitempty"`
	IndexUUID string      `json:"index_uuid,omitempty"`
	Shard     string      `json:"shard,omitempty"`
	CausedBy  *ErrorCause `json:"caused_by,omitempty"`
}

// Error is the structured error returned by search drivers.
type Error struct {
	Driver     string
	Connection string
	Operation  string
	StatusCode int
	Code       string
	Type       string
	Reason     string
	Retryable  bool
	RootCause  []ErrorCause
	Header     http.Header
	Body       []byte
	Cause      error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	detail := err.Reason
	if detail == "" && err.Cause != nil {
		detail = err.Cause.Error()
	}
	if detail == "" && err.StatusCode != 0 {
		detail = http.StatusText(err.StatusCode)
	}
	if detail == "" {
		detail = "search request failed"
	}
	prefix := err.Driver
	if prefix == "" {
		prefix = "search"
	}
	if err.Operation != "" {
		return fmt.Sprintf("%s %s: %s", prefix, err.Operation, detail)
	}
	return fmt.Sprintf("%s: %s", prefix, detail)
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// BulkError reports every failed item from a partially successful bulk request.
type BulkError struct {
	Failed []BulkItemResult
}

func (err *BulkError) Error() string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("search bulk request failed for %d item(s)", len(err.Failed))
}

func IsNotFound(err error) bool    { return hasStatus(err, http.StatusNotFound) }
func IsConflict(err error) bool    { return hasStatus(err, http.StatusConflict) }
func IsRateLimited(err error) bool { return hasStatus(err, http.StatusTooManyRequests) }

func IsRetryable(err error) bool {
	var searchErr *Error
	return errors.As(err, &searchErr) && searchErr.Retryable
}

func hasStatus(err error, status int) bool {
	var searchErr *Error
	return errors.As(err, &searchErr) && searchErr.StatusCode == status
}
