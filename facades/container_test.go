package facades

import (
	"fmt"
	"testing"

	"github.com/herhe-com/framework/contracts/config"
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

func TestRegisterAndGetServiceByType(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	cfg := &fakeConfig{values: map[string]any{"app.name": "framework"}}
	Register[config.Application](cfg)

	got, ok := Get[config.Application]()
	if !ok {
		t.Fatal("expected config service to be registered")
	}

	if got != cfg {
		t.Fatal("expected registered config service to be returned")
	}
}

func TestMustGetPanicsWhenServiceIsMissing(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	defer func() {
		if recover() == nil {
			t.Fatal("expected MustGet to panic for missing service")
		}
	}()

	_ = MustGet[config.Application]()
}

func TestRegisterNilClearsLegacyFacade(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	cfg := &fakeConfig{values: map[string]any{"app.name": "framework"}}
	Register[config.Application](cfg)
	Register[config.Application](nil)

	if got, ok := Get[config.Application](); ok || got != nil {
		t.Fatal("expected nil config service to be treated as missing")
	}
}

func TestHasReportsRegisteredService(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	cfg := &fakeConfig{values: map[string]any{"app.name": "framework"}}
	Register[config.Application](cfg)

	if !Has[config.Application]() {
		t.Fatal("expected registered config service to exist")
	}

	Register[config.Application](nil)

	if Has[config.Application]() {
		t.Fatal("expected nil config service to be treated as missing")
	}
}

func TestOptionalReturnsRegisteredService(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	cfg := &fakeConfig{values: map[string]any{"app.name": "framework"}}
	Register[config.Application](cfg)

	got, ok := Optional[config.Application]()
	if !ok {
		t.Fatal("expected optional config service to be available")
	}

	if got != cfg {
		t.Fatal("expected optional config service to match registered service")
	}
}

func TestUnregisterRemovesService(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	cfg := &fakeConfig{values: map[string]any{"app.name": "framework"}}
	Register[config.Application](cfg)

	Unregister[config.Application]()

	if Has[config.Application]() {
		t.Fatal("expected config service to be unregistered")
	}
}

func TestRootIsRegisteredByDedicatedType(t *testing.T) {
	original := Container()
	SetContainer(&Services{})
	t.Cleanup(func() {
		SetContainer(original)
	})

	Register[RootPath]("/tmp/framework")

	if got := Root(); got != "/tmp/framework" {
		t.Fatalf("expected root path to be registered, got %q", got)
	}
}
