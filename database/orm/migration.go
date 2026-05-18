package orm

import "github.com/herhe-com/framework/facades"

func migrationString(field, defaultValue string) string {
	if value := facades.Config().GetString("database.orm.migration." + field); value != "" {
		return value
	}

	return defaultValue
}

// MigrationTableName returns the configured migration table name.
func MigrationTableName() string {
	return migrationString("table", "sys_migration")
}

// MigrationDir returns the configured migration directory.
func MigrationDir() string {
	return migrationString("dir", "/migration")
}
