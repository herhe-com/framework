package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default MongoDB connection name.
func DefaultName() string {
	return facades.Config().GetString("database.mongo.default", facades.Config().GetString("database.mongodb.default", facades.Config().GetString("mongodb.default", "default")))
}

// Driver returns the configured driver name for a MongoDB connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Config().GetString("database.mongo.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Config().GetString("database.mongodb.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Config().GetString("mongodb.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Config().GetString("database.mongodb." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Config().GetString("mongodb." + name + ".driver"); driver != "" {
		return driver
	}

	return defaultValue
}

// ConnectionString returns the configured string value for a MongoDB connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("database.mongo.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("database.mongodb.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("mongodb.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("database.mongodb." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("mongodb." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionInt returns the configured int value for a MongoDB connection field.
func ConnectionInt(name, field string, defaultValue int) int {
	currentKey := "database.mongo.connections." + name + "." + field
	if facades.Config().IsSet(currentKey) {
		return facades.Config().GetInt(currentKey)
	}

	legacyKey := "database.mongodb.connections." + name + "." + field
	if facades.Config().IsSet(legacyKey) {
		return facades.Config().GetInt(legacyKey)
	}

	legacyKey = "mongodb.connections." + name + "." + field
	if facades.Config().IsSet(legacyKey) {
		return facades.Config().GetInt(legacyKey)
	}

	legacyKey = "database.mongodb." + name + "." + field
	if facades.Config().IsSet(legacyKey) {
		return facades.Config().GetInt(legacyKey)
	}

	legacyKey = "mongodb." + name + "." + field
	if facades.Config().IsSet(legacyKey) {
		return facades.Config().GetInt(legacyKey)
	}

	return defaultValue
}
