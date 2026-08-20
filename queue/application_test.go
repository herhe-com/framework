package queue

import (
	"errors"
	"fmt"
	"testing"
	"time"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	contractqueue "github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

type fakeConfig struct {
	values map[string]any
}

type fakeQueueDriver struct {
	producerKey string
	headers     contractqueue.Headers
	consumerKey string
}

func (f *fakeQueueDriver) Producer(_ []byte, key string, headers ...contractqueue.Headers) error {
	f.producerKey = key
	f.headers = contractqueue.MergeHeaders(headers...)
	return nil
}

func (f *fakeQueueDriver) Consumer(_ contractqueue.Handler, key string) error {
	f.consumerKey = key
	return nil
}

func (f *fakeQueueDriver) Close() error {
	return nil
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

func (f fakeConfig) GetInt(key string, defaultValue ...int) int {
	return 0
}

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 {
	return 0
}

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool {
	return false
}

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

func TestNewQueueWithErrorUsesLazyConnections(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{values: map[string]any{}})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	application, err := NewQueueWithError()
	if err != nil {
		t.Fatalf("NewQueueWithError() error = %v", err)
	}
	if application == nil {
		t.Fatal("NewQueueWithError() returned nil")
	}
}

func TestQueueConfiguration(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"queue.default": "rabbit",
			"queue.queues.email": map[string]any{
				"connection":   "events",
				"enable":       true,
				"topic":        "basic",
				"queue":        "basic_email",
				"routes":       []string{"email", "urgent"},
				"delay":        "3s",
				"ttl":          "2m",
				"numeric":      2,
				"retry":        3,
				"error_enable": false,
				"headers": map[string]any{
					"source": "framework",
				},
			},
			"queue.queues.disabled": map[string]any{"enable": false},
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	definition, err := queueconfig.Load("email")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if definition.Connection != "events" || !definition.Enabled {
		t.Fatalf("Load() = %#v", definition)
	}
	if got := definition.String("topic", ""); got != "basic" {
		t.Fatalf("topic = %q, want basic", got)
	}
	if got := definition.Strings("routes"); len(got) != 2 || got[1] != "urgent" {
		t.Fatalf("routes = %#v", got)
	}
	if got, err := definition.Duration("delay", 0); err != nil || got != 3*time.Second {
		t.Fatalf("delay = %s, error = %v", got, err)
	}
	if got, err := definition.Duration("ttl", 0); err != nil || got != 2*time.Minute {
		t.Fatalf("ttl = %s, error = %v", got, err)
	}
	if got, err := definition.Duration("numeric", 0); err != nil || got != 2*time.Second {
		t.Fatalf("numeric duration = %s, error = %v", got, err)
	}
	if got, err := definition.Retry(); err != nil || len(got) != 3 || got[0] != 0 {
		t.Fatalf("retry = %v, error = %v", got, err)
	}
	if got := definition.Headers()["source"]; got != "framework" {
		t.Fatalf("headers source = %v", got)
	}
	if definition.Bool("error_enable", true) {
		t.Fatal("error_enable = true, want false")
	}

	disabled, err := queueconfig.Load("disabled")
	if err != nil {
		t.Fatalf("Load(disabled) error = %v", err)
	}
	if disabled.Connection != "rabbit" {
		t.Fatalf("disabled connection = %q, want rabbit", disabled.Connection)
	}
	if !disabled.Bool("error_enable", true) {
		t.Fatal("default error_enable = false, want true")
	}
	if err = disabled.EnsureEnabled(); !errors.Is(err, contractqueue.ErrDisabled) {
		t.Fatalf("EnsureEnabled() error = %v, want ErrDisabled", err)
	}

	application, err := NewQueueWithError()
	if err != nil {
		t.Fatalf("NewQueueWithError() error = %v", err)
	}
	if err = application.Producer(nil, "disabled"); !errors.Is(err, contractqueue.ErrDisabled) {
		t.Fatalf("Producer(disabled) error = %v, want ErrDisabled", err)
	}

	driver := &fakeQueueDriver{}
	application.drivers["events"] = driver
	if err = application.Producer(nil, "email", contractqueue.Headers{"source": "runtime"}); err != nil {
		t.Fatalf("Producer(email) error = %v", err)
	}
	if driver.producerKey != "email" || driver.headers["source"] != "runtime" {
		t.Fatalf("driver producer key = %q, headers = %#v", driver.producerKey, driver.headers)
	}
	if err = application.Consumer(func([]byte) (any, error) { return nil, nil }, "email"); err != nil {
		t.Fatalf("Consumer(email) error = %v", err)
	}
	if driver.consumerKey != "email" {
		t.Fatalf("driver consumer key = %q, want email", driver.consumerKey)
	}
}
