package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default MongoDB connection name.
func DefaultName() string {
	return facades.Cfg.GetString("database.mongo.default", facades.Cfg.GetString("database.mongodb.default", facades.Cfg.GetString("mongodb.default", "default")))
}

// Driver returns the configured driver name for a MongoDB connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Cfg.GetString("database.mongo.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Cfg.GetString("database.mongodb.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Cfg.GetString("mongodb.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Cfg.GetString("database.mongodb." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Cfg.GetString("mongodb." + name + ".driver"); driver != "" {
		return driver
	}

	return defaultValue
}

// ConnectionString returns the configured string value for a MongoDB connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Cfg.GetString("database.mongo.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("database.mongodb.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("mongodb.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("database.mongodb." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("mongodb." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionInt returns the configured int value for a MongoDB connection field.
func ConnectionInt(name, field string, defaultValue int) int {
	currentKey := "database.mongo.connections." + name + "." + field
	if facades.Cfg.IsSet(currentKey) {
		return facades.Cfg.GetInt(currentKey)
	}

	legacyKey := "database.mongodb.connections." + name + "." + field
	if facades.Cfg.IsSet(legacyKey) {
		return facades.Cfg.GetInt(legacyKey)
	}

	legacyKey = "mongodb.connections." + name + "." + field
	if facades.Cfg.IsSet(legacyKey) {
		return facades.Cfg.GetInt(legacyKey)
	}

	legacyKey = "database.mongodb." + name + "." + field
	if facades.Cfg.IsSet(legacyKey) {
		return facades.Cfg.GetInt(legacyKey)
	}

	legacyKey = "mongodb." + name + "." + field
	if facades.Cfg.IsSet(legacyKey) {
		return facades.Cfg.GetInt(legacyKey)
	}

	return defaultValue
}
