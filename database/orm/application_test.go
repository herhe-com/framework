package orm

import (
	"sync"
	"testing"

	"github.com/herhe-com/framework/facades"
)

func TestDatabaseDriversCanBeLoadedConcurrently(t *testing.T) {
	originalCfg := facades.Cfg
	originalRoot := facades.Root
	facades.Root = t.TempDir() + "/"
	facades.Cfg = fakeConfig{
		values: map[string]any{
			"database.orm.default":                    "default",
			"database.orm.connections.default.driver": DriverSQLite,
			"database.orm.connections.default.path":   "default.db",
			"database.orm.connections.report.driver":  DriverSQLite,
			"database.orm.connections.report.path":    "report.db",
		},
	}
	t.Cleanup(func() {
		facades.Cfg = originalCfg
		facades.Root = originalRoot
	})

	db, err := NewApplication()
	if err != nil {
		t.Fatalf("expected database application to initialize: %v", err)
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if _, err := db.Drivers(DriverSQLite, "report"); err != nil {
				t.Errorf("expected driver, got error: %v", err)
			}
		}()
	}

	wg.Wait()
}
