package database

import (
	"github.com/redis/go-redis/v9"
)

type Redis interface {
	Default() *redis.Client

	Channel(name string) (*redis.Client, error)
}
