package queue

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	// RetryHeader stores the number of completed queue retries.
	RetryHeader = "x-retry"
	// DelayHeader stores the original delay in milliseconds.
	DelayHeader = "x-delay"
)

// ErrDisabled indicates that a queue is disabled by configuration.
var ErrDisabled = errors.New("queue is disabled")

// Handler processes a queue message body. The response is interpreted by the
// active driver. A nil response uses the driver's default acknowledgement.
// A non-nil error triggers consumer retry handling.
type Handler func(data []byte) (response any, err error)

// Headers contains transport-neutral message headers.
type Headers map[string]any

// MergeHeaders combines message headers. Later values replace earlier values.
func MergeHeaders(headers ...Headers) Headers {
	var merged Headers

	for _, values := range headers {
		for key, value := range values {
			if merged == nil {
				merged = make(Headers)
			}
			merged[key] = value
		}
	}

	return merged
}

// RetryCount returns the completed retry count stored in headers.
func RetryCount(headers Headers) int {
	count, ok := headerInt64(headers, RetryHeader)
	if !ok || count < 0 || uint64(count) > uint64(^uint(0)>>1) {
		return 0
	}

	return int(count)
}

// WithRetryCount returns a header copy containing the completed retry count.
func WithRetryCount(headers Headers, count int) Headers {
	cloned := make(Headers, len(headers)+1)
	for key, value := range headers {
		cloned[key] = value
	}
	if count < 0 {
		count = 0
	}
	cloned[RetryHeader] = count

	return cloned
}

// RetryDelay returns the original delayed-message duration stored in headers.
func RetryDelay(headers Headers) time.Duration {
	milliseconds, ok := headerInt64(headers, DelayHeader)
	if !ok {
		return 0
	}
	if milliseconds < 0 {
		if milliseconds == -1<<63 {
			return 0
		}
		milliseconds = -milliseconds
	}
	if milliseconds > int64(time.Duration(1<<63-1)/time.Millisecond) {
		return 0
	}

	return time.Duration(milliseconds) * time.Millisecond
}

// RetryAfter returns the wait before the next requeue for a message that has
// already completed retried attempts. ok is false when retries are exhausted.
// The length of retry is the maximum number of requeues.
func RetryAfter(retry []time.Duration, retried int) (wait time.Duration, ok bool) {
	if retried < 0 || retried >= len(retry) {
		return 0, false
	}

	return retry[retried], true
}

// HasPositiveRetryDelay reports whether any configured retry step waits.
func HasPositiveRetryDelay(retry []time.Duration) bool {
	for _, wait := range retry {
		if wait > 0 {
			return true
		}
	}

	return false
}

func headerInt64(headers Headers, key string) (int64, bool) {
	value, exists := headers[key]
	if !exists || value == nil {
		return 0, false
	}

	switch values := value.(type) {
	case []string:
		if len(values) == 0 {
			return 0, false
		}
		value = values[0]
	case []byte:
		value = string(values)
	}

	parsed, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
	return parsed, err == nil
}

type Queue interface {
	Driver
	Channel(name string) (Driver, error)
}

type Driver interface {
	Producer(body []byte, key string, headers ...Headers) error
	Consumer(handler Handler, key string) error
	Close() error
}
