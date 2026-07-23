package config

import "github.com/herhe-com/framework/facades"

// DefaultName returns the configured default queue connection name.
func DefaultName() string {
	return facades.Config().GetString("queue.default", "default")
}

// Driver returns the configured driver for a queue connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Config().GetString("queue.connections." + name + ".driver"); driver != "" {
		return driver
	}

	for _, driver := range []string{"rabbitmq", "nats"} {
		if value := facades.Config().GetString("queue." + driver + "." + name + ".driver"); value != "" {
			return value
		}
	}

	return defaultValue
}

// ConnectionString returns the configured string value for a queue connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("queue.connections." + name + "." + field); value != "" {
		return value
	}

	for _, driver := range []string{"rabbitmq", "nats"} {
		if value := facades.Config().GetString("queue." + driver + "." + name + "." + field); value != "" {
			return value
		}
	}

	return defaultValue
}
