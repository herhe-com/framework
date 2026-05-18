package orm

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

func TestMigrationConfigPrefersNewNamespace(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"database.orm.migration.table": "new_table",
			"database.orm.migration.dir":   "/new/migration",
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if got := MigrationTableName(); got != "new_table" {
		t.Fatalf("expected new namespace table, got %q", got)
	}

	if got := MigrationDir(); got != "/new/migration" {
		t.Fatalf("expected new namespace dir, got %q", got)
	}
}
