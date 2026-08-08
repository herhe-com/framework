package config

import "github.com/herhe-com/framework/facades"

// ConnectionString returns the configured string value for a search connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("search.connections." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionStrings returns the configured string slice value for a search connection field.
func ConnectionStrings(name, field string, defaultValue []string) []string {
	if values := facades.Config().GetStrings("search.connections." + name + "." + field); len(values) > 0 {
		return values
	}

	return defaultValue
}
