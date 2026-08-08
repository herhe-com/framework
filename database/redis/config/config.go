package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default redis connection name.
func DefaultName() string {
	return facades.Config().GetString("database.redis.default", "default")
}

// ConnectionString returns the configured string value for a redis connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("database.redis.connections." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionInt returns the configured int value for a redis connection field.
func ConnectionInt(name, field string, defaultValue int) int {
	key := "database.redis.connections." + name + "." + field
	if facades.Config().IsSet(key) {
		return facades.Config().GetInt(key)
	}
	return defaultValue
}
