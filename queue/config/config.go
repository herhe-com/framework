package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default queue connection name.
func DefaultName() string {
	return facades.Cfg.GetString("queue.default", "default")
}

// Driver returns the configured driver for a queue connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Cfg.GetString("queue.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if driver := facades.Cfg.GetString("queue.rabbitmq." + name + ".driver"); driver != "" {
		return driver
	}

	return defaultValue
}

// ConnectionString returns the configured string value for a queue connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Cfg.GetString("queue.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("queue.rabbitmq." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}
