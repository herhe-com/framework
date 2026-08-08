package queue

import (
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

// Handler processes a queue message body. A non-nil error triggers consumer retry handling.
type Handler func(data []byte) error

// Headers contains transport-neutral message headers.
type Headers map[string]any

// ProducerOptions describes where and how a message is published.
type ProducerOptions struct {
	Topic   string
	Queue   string
	Routes  []string
	Delay   time.Duration
	TTL     time.Duration
	Headers Headers
}

// ConsumerOptions describes a queue subscription.
type ConsumerOptions struct {
	Topic   string
	Queue   string
	Route   string
	Delayed bool
	TTL     time.Duration
	Retry   int
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
	Producer(body []byte, options ProducerOptions) error
	Consumer(handler Handler, options ConsumerOptions) error
	Close() error
}
