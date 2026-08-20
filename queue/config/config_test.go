package config

import (
	"fmt"
	"testing"
	"time"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/facades"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Env(key string, defaultValue ...any) any {
	return f.Get(key, defaultValue...)
}

func (f fakeConfig) Add(name string, configuration map[string]any) {}

func (f fakeConfig) Set(key string, configuration any) {}

func (f fakeConfig) Get(key string, defaultValue ...any) any {
	if value, ok := f.values[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

func (f fakeConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := f.values[key]; ok {
		return fmt.Sprint(value)
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func (f fakeConfig) GetStrings(key string, defaultValue ...[]string) []string {
	return nil
}

func (f fakeConfig) GetMaps(key string, defaultValue ...map[string]any) map[string]any {
	return nil
}

func (f fakeConfig) GetInt(key string, defaultValue ...int) int { return 0 }

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 { return 0 }

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool { return false }

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

func withConfig(t *testing.T, values map[string]any) {
	t.Helper()
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{values: values})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})
}

func TestRetryConfig(t *testing.T) {
	withConfig(t, map[string]any{
		"queue.default": "rabbit",
		"queue.queues.email": map[string]any{
			"retry": []any{"1m", "5m", "30m"},
		},
		"queue.queues.legacy": map[string]any{
			"retry": 3,
		},
		"queue.queues.single": map[string]any{
			"retry": "10s",
		},
		"queue.queues.disabled": map[string]any{
			"retry": false,
		},
		"queue.queues.disabled_text": map[string]any{
			"retry": "false",
		},
		"queue.queues.empty": map[string]any{},
	})

	email, err := Load("email")
	if err != nil {
		t.Fatalf("Load(email) error = %v", err)
	}
	got, err := email.Retry()
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}
	if len(got) != len(want) {
		t.Fatalf("Retry() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Retry()[%d] = %s, want %s", i, got[i], want[i])
		}
	}

	legacy, err := Load("legacy")
	if err != nil {
		t.Fatalf("Load(legacy) error = %v", err)
	}
	got, err = legacy.Retry()
	if err != nil {
		t.Fatalf("legacy Retry() error = %v", err)
	}
	if len(got) != 3 || got[0] != 0 || got[2] != 0 {
		t.Fatalf("legacy Retry() = %v, want three zero delays", got)
	}

	single, err := Load("single")
	if err != nil {
		t.Fatalf("Load(single) error = %v", err)
	}
	got, err = single.Retry()
	if err != nil {
		t.Fatalf("single Retry() error = %v", err)
	}
	if len(got) != 1 || got[0] != 10*time.Second {
		t.Fatalf("single Retry() = %v, want [10s]", got)
	}

	for _, key := range []string{"disabled", "disabled_text"} {
		disabled, err := Load(key)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", key, err)
		}
		got, err = disabled.Retry()
		if err != nil {
			t.Fatalf("%s Retry() error = %v", key, err)
		}
		if got != nil {
			t.Fatalf("%s Retry() = %v, want nil", key, got)
		}
	}

	empty, err := Load("empty")
	if err != nil {
		t.Fatalf("Load(empty) error = %v", err)
	}
	got, err = empty.Retry()
	if err != nil {
		t.Fatalf("empty Retry() error = %v", err)
	}
	if got != nil {
		t.Fatalf("empty Retry() = %v, want nil", got)
	}
}

func TestRetryConfigInvalid(t *testing.T) {
	withConfig(t, map[string]any{
		"queue.default": "rabbit",
		"queue.queues.bad": map[string]any{
			"retry": []any{"1m", "nope"},
		},
		"queue.queues.enabled_true": map[string]any{
			"retry": true,
		},
	})

	definition, err := Load("bad")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err = definition.Retry(); err == nil {
		t.Fatal("Retry() error = nil, want invalid duration")
	}

	enabled, err := Load("enabled_true")
	if err != nil {
		t.Fatalf("Load(enabled_true) error = %v", err)
	}
	if _, err = enabled.Retry(); err == nil {
		t.Fatal("Retry(true) error = nil, want unsupported")
	}
}
