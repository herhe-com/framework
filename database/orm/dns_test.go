package orm

import (
	"net/url"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMysqlDSNIncludesConnectionOptions(t *testing.T) {
	dsn := mysqlDSN("root", "p@ss word", "127.0.0.1", "3306", "upper", "utf8mb4")

	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("expected valid mysql DSN, got %q: %v", dsn, err)
	}

	if cfg.User != "root" || cfg.Passwd != "p@ss word" || cfg.Addr != "127.0.0.1:3306" || cfg.DBName != "upper" {
		t.Fatalf("unexpected mysql DSN config: %#v", cfg)
	}

	if !strings.Contains(dsn, "charset=utf8mb4") || !cfg.ParseTime || cfg.Loc.String() != "Local" {
		t.Fatalf("expected charset, parseTime and Local timezone, got %q", dsn)
	}
}

func TestPostgreDSNUsesURLFormat(t *testing.T) {
	dsn := postgreDSN("postgres", "p@ss word", "127.0.0.1", "5432", "upper", "disable", "Asia/Shanghai")

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("expected valid postgresql DSN URL, got %q: %v", dsn, err)
	}

	password, _ := parsed.User.Password()
	if parsed.Scheme != "postgres" || parsed.User.Username() != "postgres" || password != "p@ss word" {
		t.Fatalf("unexpected postgresql user info in %q", dsn)
	}

	if parsed.Host != "127.0.0.1:5432" || parsed.Path != "/upper" {
		t.Fatalf("unexpected postgresql host/path in %q", dsn)
	}

	if parsed.Query().Get("sslmode") != "disable" || parsed.Query().Get("TimeZone") != "Asia/Shanghai" {
		t.Fatalf("unexpected postgresql query in %q", dsn)
	}
}

func TestSQLServerDSNEscapesPassword(t *testing.T) {
	dsn := sqlserverDSN("sa", "p@ss word", "127.0.0.1", "1433", "upper")

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("expected valid sqlserver DSN URL, got %q: %v", dsn, err)
	}

	password, _ := parsed.User.Password()
	if parsed.Scheme != "sqlserver" || parsed.User.Username() != "sa" || password != "p@ss word" {
		t.Fatalf("unexpected sqlserver user info in %q", dsn)
	}

	if parsed.Host != "127.0.0.1:1433" || parsed.Query().Get("database") != "upper" {
		t.Fatalf("unexpected sqlserver host/query in %q", dsn)
	}
}

func TestSQLServerDSNEscapesDatabaseName(t *testing.T) {
	dsn := sqlserverDSN("sa", "secret", "127.0.0.1", "1433", "upper&report")

	if !strings.Contains(dsn, "database=upper%26report") {
		t.Fatalf("expected escaped sqlserver database query, got %q", dsn)
	}
}
