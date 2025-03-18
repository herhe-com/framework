package config

type Application interface {
	Env(key string, defaultValue ...any) any
	Add(name string, configuration map[string]any)
	Set(key string, configuration any)
	Get(key string, defaultValue ...any) any
	GetString(key string, defaultValue ...string) string
	GetStrings(key string, defaultValue ...[]string) []string
	GetMaps(key string, defaultValue ...map[string]any) map[string]any
	GetInt(key string, defaultValue ...int) int
	GetInt64(key string, defaultValue ...int64) int64
	GetBool(key string, defaultValue ...bool) bool
}
