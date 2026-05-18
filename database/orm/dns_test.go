package orm

import (
	"strings"
	"testing"
)

func TestSQLServerDSNEscapesPassword(t *testing.T) {
	dsn := sqlserverDSN("sa", "p@ss word", "127.0.0.1", "1433", "upper")

	if !strings.Contains(dsn, "sqlserver://sa:p%40ss+word@127.0.0.1:1433?database=upper") {
		t.Fatalf("expected escaped sqlserver DSN, got %q", dsn)
	}
}
