package validation

import (
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
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

func TestErrorsGroupsValidationMessagesByField(t *testing.T) {
	originalCfg := facades.Cfg
	originalValidator := facades.Validator
	facades.Cfg = fakeConfig{
		values: map[string]any{
			"app.language":     "en",
			"validation.label": "label",
		},
	}
	NewApplication()
	t.Cleanup(func() {
		facades.Cfg = originalCfg
		facades.Validator = originalValidator
	})

	type payload struct {
		Name string `label:"name" validate:"required"`
	}

	err := facades.Validator.Struct(payload{})
	if err == nil {
		t.Fatal("expected validation to fail")
	}

	messages := Errors(err.(validator.ValidationErrors))
	if len(messages["name"]) != 1 {
		t.Fatalf("expected one validation message for name, got %#v", messages)
	}
}
