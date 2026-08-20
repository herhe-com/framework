package config

import (
	"fmt"
	"strings"
	"time"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
)

// Queue contains one queue.queues.<key> configuration entry.
type Queue struct {
	Key        string
	Connection string
	Enabled    bool
	values     map[string]any
}

// DefaultName returns the configured default queue connection name.
func DefaultName() string {
	return facades.Config().GetString("queue.default", "default")
}

// Load returns a queue definition by key.
func Load(key string) (Queue, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Queue{}, fmt.Errorf("queue key is required")
	}

	configKey := "queue.queues." + key
	values, ok := facades.Config().Get(configKey).(map[string]any)
	if !ok || values == nil {
		return Queue{}, fmt.Errorf("queue configuration not found: %s", key)
	}

	enabled := true
	if value, exists := values["enable"]; exists {
		enabled = cast.ToBool(value)
	}

	connection := strings.TrimSpace(cast.ToString(values["connection"]))
	if connection == "" {
		connection = DefaultName()
	}

	return Queue{
		Key:        key,
		Connection: connection,
		Enabled:    enabled,
		values:     values,
	}, nil
}

// EnsureEnabled returns ErrDisabled when this queue is switched off.
func (r Queue) EnsureEnabled() error {
	if r.Enabled {
		return nil
	}

	return fmt.Errorf("%w: %s", contractqueue.ErrDisabled, r.Key)
}

// Value returns a driver-specific queue configuration value.
func (r Queue) Value(field string) any {
	return r.values[field]
}

// String returns a string queue configuration value.
func (r Queue) String(field, defaultValue string) string {
	value := strings.TrimSpace(cast.ToString(r.Value(field)))
	if value == "" {
		return defaultValue
	}

	return value
}

// Strings returns a string slice queue configuration value.
func (r Queue) Strings(field string) []string {
	return cast.ToStringSlice(r.Value(field))
}

// Bool returns a boolean queue configuration value.
func (r Queue) Bool(field string, defaultValue bool) bool {
	value := r.Value(field)
	if value == nil {
		return defaultValue
	}

	return cast.ToBool(value)
}

// Int returns an integer queue configuration value.
func (r Queue) Int(field string, defaultValue int) int {
	value := r.Value(field)
	if value == nil {
		return defaultValue
	}

	return cast.ToInt(value)
}

// Duration returns a duration queue configuration value.
func (r Queue) Duration(field string, defaultValue time.Duration) (time.Duration, error) {
	value := r.Value(field)
	if value == nil {
		return defaultValue, nil
	}
	if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
		return defaultValue, nil
	}

	duration, err := parseDurationValue(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for queue.queues.%s.%s: %w", r.Key, field, err)
	}

	return duration, nil
}

// Retry returns configured consumer retry delays. The slice length is the
// maximum number of requeues. A legacy integer N expands to N zero delays.
// retry: false explicitly disables requeues (empty result).
func (r Queue) Retry() ([]time.Duration, error) {
	value := r.Value("retry")
	if value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case bool:
		if !typed {
			return nil, nil
		}
		return nil, fmt.Errorf("invalid retry for queue.queues.%s.retry: true is not supported, use a duration list or count", r.Key)
	case []time.Duration:
		result, err := cloneDurations(typed)
		if err != nil {
			return nil, fmt.Errorf("invalid duration for queue.queues.%s.retry: %w", r.Key, err)
		}
		return result, nil
	case time.Duration:
		if typed < 0 {
			return nil, fmt.Errorf("invalid duration for queue.queues.%s.retry: must not be negative", r.Key)
		}
		return []time.Duration{typed}, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		count := cast.ToInt(typed)
		if count < 0 {
			return nil, fmt.Errorf("invalid retry count for queue.queues.%s.retry: must not be negative", r.Key)
		}
		return make([]time.Duration, count), nil
	case float32, float64:
		// YAML/JSON numbers may decode as float; only whole counts are allowed.
		asFloat := cast.ToFloat64(typed)
		count := int(asFloat)
		if asFloat != float64(count) || count < 0 {
			return nil, fmt.Errorf("invalid retry count for queue.queues.%s.retry: must be a non-negative integer", r.Key)
		}
		return make([]time.Duration, count), nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil
		}
		if strings.EqualFold(text, "false") {
			return nil, nil
		}
		if strings.EqualFold(text, "true") {
			return nil, fmt.Errorf("invalid retry for queue.queues.%s.retry: true is not supported, use a duration list or count", r.Key)
		}
		// A bare integer string is a legacy retry count; duration strings are steps.
		if count, err := cast.ToIntE(text); err == nil {
			if count < 0 {
				return nil, fmt.Errorf("invalid retry count for queue.queues.%s.retry: must not be negative", r.Key)
			}
			return make([]time.Duration, count), nil
		}
		duration, err := parseDurationValue(text)
		if err != nil {
			return nil, fmt.Errorf("invalid duration for queue.queues.%s.retry: %w", r.Key, err)
		}
		return []time.Duration{duration}, nil
	case []string:
		return parseDurationSlice(r.Key, typed)
	case []any:
		return parseDurationSlice(r.Key, typed)
	default:
		// Fallback for other slice-like config values.
		if slice := cast.ToStringSlice(value); len(slice) > 0 || isEmptySlice(value) {
			return parseDurationSlice(r.Key, slice)
		}
		return nil, fmt.Errorf("invalid retry for queue.queues.%s.retry: unsupported type %T", r.Key, value)
	}
}

func parseDurationSlice[T any](key string, values []T) ([]time.Duration, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make([]time.Duration, 0, len(values))
	for index, item := range values {
		duration, err := parseDurationValue(item)
		if err != nil {
			return nil, fmt.Errorf("invalid duration for queue.queues.%s.retry[%d]: %w", key, index, err)
		}
		result = append(result, duration)
	}

	return result, nil
}

func parseDurationValue(value any) (time.Duration, error) {
	if value == nil {
		return 0, fmt.Errorf("duration is required")
	}

	switch typed := value.(type) {
	case time.Duration:
		if typed < 0 {
			return 0, fmt.Errorf("must not be negative")
		}
		return typed, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		value = fmt.Sprintf("%vs", typed)
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, fmt.Errorf("duration is required")
		}
		value = text
	}

	duration, err := cast.ToDurationE(value)
	if err != nil {
		return 0, err
	}
	if duration < 0 {
		return 0, fmt.Errorf("must not be negative")
	}

	return duration, nil
}

func cloneDurations(values []time.Duration) ([]time.Duration, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make([]time.Duration, len(values))
	for index, duration := range values {
		if duration < 0 {
			return nil, fmt.Errorf("must not be negative")
		}
		result[index] = duration
	}

	return result, nil
}

func isEmptySlice(value any) bool {
	switch typed := value.(type) {
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case []time.Duration:
		return len(typed) == 0
	default:
		return false
	}
}

// Headers returns configured message headers.
func (r Queue) Headers() contractqueue.Headers {
	values := cast.ToStringMap(r.Value("headers"))
	if len(values) == 0 {
		return nil
	}

	return contractqueue.Headers(values)
}
