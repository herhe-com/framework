package search

import (
	"fmt"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"
	contractsearch "github.com/herhe-com/framework/contracts/search"
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
	if value, ok := f.values[key]; ok {
		if strings, ok := value.([]string); ok {
			return strings
		}
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

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

func TestNewSearchWithErrorReturnsConfigError(t *testing.T) {
	originalCfg := facades.Cfg
	facades.Cfg = fakeConfig{values: map[string]any{}}
	t.Cleanup(func() {
		facades.Cfg = originalCfg
	})

	search, err := NewSearchWithError()
	if err == nil {
		t.Fatal("expected missing default search driver to return an error")
	}

	if search != nil {
		t.Fatal("expected search to be nil when initialization fails")
	}
}

func TestSearchChannelCanBeLoadedConcurrently(t *testing.T) {
	originalCfg := facades.Cfg
	originalValidator := facades.Validator
	facades.Cfg = fakeConfig{
		values: map[string]any{
			"search.default":                      "default",
			"search.connections.default.driver":   contractsearch.DriverMeiliSearch,
			"search.connections.default.host":     "http://127.0.0.1:7700",
			"search.connections.default.secret":   "masterKey",
			"search.connections.secondary.driver": contractsearch.DriverMeiliSearch,
			"search.connections.secondary.host":   "http://127.0.0.1:7700",
			"search.connections.secondary.secret": "masterKey",
			"search.connections.secondary.prefix": "test_",
		},
	}
	facades.Validator = validator.New()
	t.Cleanup(func() {
		facades.Cfg = originalCfg
		facades.Validator = originalValidator
	})

	app, err := NewSearchWithError()
	if err != nil {
		t.Fatalf("expected search application to initialize: %v", err)
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if _, err := app.Channel(contractsearch.DriverMeiliSearch, "secondary"); err != nil {
				t.Errorf("expected driver, got error: %v", err)
			}
		}()
	}

	wg.Wait()
}
