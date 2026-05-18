package auth

import (
	"fmt"
	"testing"

	contractauth "github.com/herhe-com/framework/contracts/auth"
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

func TestNewJWTokenCanBeChecked(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"app.name":   "framework",
			"jwt.sub":    "api",
			"jwt.secret": "test-secret",
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	token, err := NewJWToken("user-1", 5, true, map[string]any{"role": "admin"})
	if err != nil {
		t.Fatalf("expected token to be created: %v", err)
	}

	var claims contractauth.Claims
	refresh, err := CheckJWToken(&claims, token)
	if err != nil {
		t.Fatalf("expected token to be valid: %v", err)
	}

	if refresh {
		t.Fatal("expected valid token to not require refresh")
	}

	if claims.Subject != "user-1" {
		t.Fatalf("expected subject user-1, got %q", claims.Subject)
	}

	if claims.Issuer != "framework:api" {
		t.Fatalf("expected issuer framework:api, got %q", claims.Issuer)
	}
}
