package config

import "github.com/herhe-com/framework/facades"

// ConnectionString returns the configured string value for a search connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString("search.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("search.elasticsearch." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Config().GetString("search.meilisearch." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionStrings returns the configured string slice value for a search connection field.
func ConnectionStrings(name, field string, defaultValue []string) []string {
	if values := facades.Config().GetStrings("search.connections." + name + "." + field); len(values) > 0 {
		return values
	}

	if values := facades.Config().GetStrings("search.elasticsearch." + name + "." + field); len(values) > 0 {
		return values
	}

	if values := facades.Config().GetStrings("search.meilisearch." + name + "." + field); len(values) > 0 {
		return values
	}

	return defaultValue
}

// Driver returns the configured driver name for a search connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Config().GetString("search.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if cfg, ok := facades.Config().Get("search.elasticsearch." + name).(map[string]any); ok && len(cfg) > 0 {
		return "elasticsearch"
	}

	if cfg, ok := facades.Config().Get("search.meilisearch." + name).(map[string]any); ok && len(cfg) > 0 {
		return "meilisearch"
	}

	return defaultValue
}
