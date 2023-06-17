package config

type Application interface {
	Env(key string, defaultValue ...any) any
	Add(name string, configuration map[string]any)
	Get(key string, defaultValue ...any) any
	GetString(key string, defaultValue ...string) string
	GetInt(key string, defaultValue ...int) int
	GetInt64(key string, defaultValue ...int64) int64
	GetBool(key string, defaultValue ...bool) bool
}
