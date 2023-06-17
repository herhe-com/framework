package redis

import (
	"github.com/herhe-com/framework/facades"
	"github.com/redis/go-redis/v9"
	"net"
)

func NewApplication() (err error) {

	addr := net.JoinHostPort(facades.Cfg.GetString("database.redis.host"), facades.Cfg.GetString("database.redis.port"))

	facades.Redis = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: facades.Cfg.GetString("database.redis.password"),
		DB:       facades.Cfg.GetInt("database.redis.database"),
	})

	return nil
}
