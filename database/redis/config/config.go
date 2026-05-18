package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default redis connection name.
func DefaultName() string {
	return facades.Config().GetString("database.redis.default", "default")
}

// Driver returns the configured driver for a redis connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Config().GetString("database.redis.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Config().GetString("database.redis." + name + ".driver"); driver != "" {
		return driver
	}

	return defaultValue
}

// ConnectionString returns the configured string value for a redis connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("database.redis.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("database.redis." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionInt returns the configured int value for a redis connection field.
func ConnectionInt(name, field string, defaultValue int) int {
	currentKey := "database.redis.connections." + name + "." + field
	if facades.Config().IsSet(currentKey) {
		return facades.Config().GetInt(currentKey)
	}

	legacyKey := "database.redis." + name + "." + field
	if facades.Config().IsSet(legacyKey) {
		return facades.Config().GetInt(legacyKey)
	}

	return defaultValue
}
