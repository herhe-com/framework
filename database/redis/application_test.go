package redis

import (
	"fmt"
	"testing"

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
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (f fakeConfig) GetMaps(key string, defaultValue ...map[string]any) map[string]any {
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (f fakeConfig) GetInt(key string, defaultValue ...int) int {
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return 0
}

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 {
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return 0
}

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool {
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return false
}

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

func TestNewRedisClientReadsDriverFromConnectionConfig(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"database.redis.connections.default": map[string]any{
				"driver": "unsupported",
			},
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	_, _, err := newRedisClient("default")
	if err == nil {
		t.Fatal("expected invalid redis driver to return an error")
	}

	if got, want := err.Error(), "invalid driver: unsupported, only support redis"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
