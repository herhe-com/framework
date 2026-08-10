package orm

import (
	"testing"
	"time"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/facades"
)

func registerORMTestConfig(t *testing.T, values map[string]any) {
	t.Helper()

	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{values: values})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})
}

func TestORMPoolConfigUsesDefaults(t *testing.T) {
	registerORMTestConfig(t, nil)

	got, err := ormPoolConfigOf("default")
	if err != nil {
		t.Fatalf("expected default pool config, got error: %v", err)
	}

	if got.maxOpenConns != defaultORMMaxOpenConns || got.maxIdleConns != defaultORMMaxIdleConns {
		t.Fatalf("unexpected default pool sizes: %#v", got)
	}

	if got.connMaxLifetime != defaultORMConnMaxLifetime || got.connMaxIdleTime != defaultORMConnMaxIdleTime {
		t.Fatalf("unexpected default pool durations: %#v", got)
	}
}

func TestORMPoolConfigReadsConnectionOverrides(t *testing.T) {
	registerORMTestConfig(t, map[string]any{
		"database.orm.connections.default.pool.max_open_conns":     25,
		"database.orm.connections.default.pool.max_idle_conns":     "6",
		"database.orm.connections.default.pool.conn_max_lifetime":  "45m",
		"database.orm.connections.default.pool.conn_max_idle_time": "7m",
	})

	got, err := ormPoolConfigOf("default")
	if err != nil {
		t.Fatalf("expected configured pool, got error: %v", err)
	}

	if got.maxOpenConns != 25 || got.maxIdleConns != 6 {
		t.Fatalf("unexpected configured pool sizes: %#v", got)
	}

	if got.connMaxLifetime != 45*time.Minute || got.connMaxIdleTime != 7*time.Minute {
		t.Fatalf("unexpected configured pool durations: %#v", got)
	}
}

func TestORMPoolConfigRejectsInvalidDuration(t *testing.T) {
	registerORMTestConfig(t, map[string]any{
		"database.orm.connections.default.pool.conn_max_lifetime": "not-a-duration",
	})

	if _, err := ormPoolConfigOf("default"); err == nil {
		t.Fatal("expected invalid duration to return an error")
	}
}

func TestSQLiteClientAppliesPoolConfig(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir() + "/"))
	facades.Register[contractconfig.Application](fakeConfig{values: map[string]any{
		"database.orm.connections.default": map[string]any{
			"driver": DriverSQLite,
		},
		"database.orm.connections.default.path":                    "pool.db",
		"database.orm.connections.default.pool.max_open_conns":     7,
		"database.orm.connections.default.pool.max_idle_conns":     3,
		"database.orm.connections.default.pool.conn_max_lifetime":  "2m",
		"database.orm.connections.default.pool.conn_max_idle_time": "1m",
	}})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	db, _, err := newSQLiteClient("default")
	if err != nil {
		t.Fatalf("expected sqlite client, got error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expected database/sql pool, got error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("expected max open connections 7, got %d", got)
	}
}
