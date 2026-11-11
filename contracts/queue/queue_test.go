package queue

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRetryHeaders(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
	}{
		{name: "int", value: 1, want: 1},
		{name: "int32", value: int32(2), want: 2},
		{name: "int64", value: int64(3), want: 3},
		{name: "string", value: "4", want: 4},
		{name: "NATS values", value: []string{"5"}, want: 5},
		{name: "invalid", value: "invalid"},
		{name: "negative", value: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RetryCount(Headers{RetryHeader: tt.value}); got != tt.want {
				t.Fatalf("RetryCount() = %d, want %d", got, tt.want)
			}
		})
	}

	original := Headers{"source": "test"}
	got := WithRetryCount(original, 2)
	if !reflect.DeepEqual(got, Headers{"source": "test", RetryHeader: 2}) {
		t.Fatalf("WithRetryCount() = %v", got)
	}
	if _, exists := original[RetryHeader]; exists {
		t.Fatal("WithRetryCount() mutated the original headers")
	}
}

func TestRetryDelay(t *testing.T) {
	for _, value := range []any{int64(1500), int64(-1500), "1500"} {
		if got := RetryDelay(Headers{DelayHeader: value}); got != 1500*time.Millisecond {
			t.Fatalf("RetryDelay(%v) = %s, want %s", value, got, 1500*time.Millisecond)
		}
	}
}

func TestMergeHeaders(t *testing.T) {
	got := MergeHeaders(
		Headers{"source": "config", "keep": true},
		Headers{"source": "runtime"},
	)
	want := Headers{"source": "runtime", "keep": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeHeaders() = %#v, want %#v", got, want)
	}
}

func TestRetryAfter(t *testing.T) {
	retry := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}

	for _, tt := range []struct {
		retried int
		wait    time.Duration
		ok      bool
	}{
		{retried: 0, wait: time.Minute, ok: true},
		{retried: 1, wait: 5 * time.Minute, ok: true},
		{retried: 2, wait: 30 * time.Minute, ok: true},
		{retried: 3, ok: false},
		{retried: -1, ok: false},
	} {
		wait, ok := RetryAfter(retry, tt.retried)
		if ok != tt.ok || wait != tt.wait {
			t.Fatalf("RetryAfter(%d) = (%s, %v), want (%s, %v)", tt.retried, wait, ok, tt.wait, tt.ok)
		}
	}

	if _, ok := RetryAfter(nil, 0); ok {
		t.Fatal("RetryAfter(nil) ok = true, want false")
	}
}

func TestHasPositiveRetryDelay(t *testing.T) {
	if HasPositiveRetryDelay(nil) {
		t.Fatal("HasPositiveRetryDelay(nil) = true")
	}
	if HasPositiveRetryDelay([]time.Duration{0, 0}) {
		t.Fatal("HasPositiveRetryDelay(zeros) = true")
	}
	if !HasPositiveRetryDelay([]time.Duration{0, time.Second}) {
		t.Fatal("HasPositiveRetryDelay(mixed) = false")
	}
}

func TestCallHandler(t *testing.T) {
	t.Run("nil handler", func(t *testing.T) {
		if _, err := CallHandler("test", nil, nil, time.Second); err == nil {
			t.Fatal("CallHandler() error = nil, want an error")
		}
	})

	t.Run("unlimited", func(t *testing.T) {
		response, err := CallHandler("test", func([]byte) (any, error) {
			time.Sleep(50 * time.Millisecond)

			return "ok", nil
		}, nil, 0)
		if err != nil || response != "ok" {
			t.Fatalf("CallHandler() = (%v, %v), want (ok, nil)", response, err)
		}
	})

	t.Run("fast handler", func(t *testing.T) {
		response, err := CallHandler("test", func([]byte) (any, error) {
			return "ok", nil
		}, nil, time.Second)
		if err != nil || response != "ok" {
			t.Fatalf("CallHandler() = (%v, %v), want (ok, nil)", response, err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		start := time.Now()
		response, err := CallHandler("test", func([]byte) (any, error) {
			time.Sleep(500 * time.Millisecond)

			return "late", nil
		}, nil, 20*time.Millisecond)

		if !errors.Is(err, ErrHandlerTimeout) {
			t.Fatalf("CallHandler() error = %v, want ErrHandlerTimeout", err)
		}
		if !strings.Contains(err.Error(), "timed out after 20ms") {
			t.Fatalf("CallHandler() error = %v, want the timeout duration", err)
		}
		if response != nil {
			t.Fatalf("CallHandler() response = %v, want nil", response)
		}
		if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
			t.Fatalf("CallHandler() blocked for %s, want it to return at the timeout", elapsed)
		}
	})

	t.Run("handler error wins", func(t *testing.T) {
		_, err := CallHandler("test", func([]byte) (any, error) {
			return nil, errors.New("boom")
		}, nil, time.Second)
		if err == nil || err.Error() != "boom" {
			t.Fatalf("CallHandler() error = %v, want boom", err)
		}
	})
}
