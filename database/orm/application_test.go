package orm

import (
	"sync"
	"testing"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/facades"
)

func TestDatabaseDriversCanBeLoadedConcurrently(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir() + "/"))
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"database.orm.default":                    "default",
			"database.orm.connections.default.driver": DriverSQLite,
			"database.orm.connections.default.path":   "default.db",
			"database.orm.connections.report.driver":  DriverSQLite,
			"database.orm.connections.report.path":    "report.db",
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
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

func TestResolveDatabaseDriverSupportsSQLServerDefaultConnection(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{
		values: map[string]any{
			"database.orm.default":                    "default",
			"database.orm.connections.default.driver": DriverSQLServer,
		},
	})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if got := resolveDatabaseDriver("", "default"); got != DriverSQLServer {
		t.Fatalf("expected sqlserver driver, got %q", got)
	}
}

func TestMysqlCharsetDefaultIsUtf8mb4(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if got := mysqlCharset("default"); got != "utf8mb4" {
		t.Fatalf("expected default mysql charset utf8mb4, got %q", got)
	}
}
