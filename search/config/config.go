package config

import "github.com/herhe-com/framework/facades"

// ConnectionString returns the configured string value for a search connection field.
func ConnectionString(name, field, defaultValue string) string {
	if value := facades.Cfg.GetString("search.connections." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("search.elasticsearch." + name + "." + field); value != "" {
		return value
	}

	if value := facades.Cfg.GetString("search.meilisearch." + name + "." + field); value != "" {
		return value
	}

	return defaultValue
}

// ConnectionStrings returns the configured string slice value for a search connection field.
func ConnectionStrings(name, field string, defaultValue []string) []string {
	if values := facades.Cfg.GetStrings("search.connections." + name + "." + field); len(values) > 0 {
		return values
	}

	if values := facades.Cfg.GetStrings("search.elasticsearch." + name + "." + field); len(values) > 0 {
		return values
	}

	if values := facades.Cfg.GetStrings("search.meilisearch." + name + "." + field); len(values) > 0 {
		return values
	}

	return defaultValue
}

// Driver returns the configured driver name for a search connection.
func Driver(name, defaultValue string) string {
	if driver := facades.Cfg.GetString("search.connections." + name + ".driver"); driver != "" {
		return driver
	}

	if cfg, ok := facades.Cfg.Get("search.elasticsearch." + name).(map[string]any); ok && len(cfg) > 0 {
		return "elasticsearch"
	}

	if cfg, ok := facades.Cfg.Get("search.meilisearch." + name).(map[string]any); ok && len(cfg) > 0 {
		return "meilisearch"
	}

	return defaultValue
}
