package locker

import (
	"errors"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() (err error) {

	cache, ok := facades.OptionalRedis()
	if !ok {
		return errors.New("please initialize Redis first")
	}

	pool := goredis.NewPool(cache.Default())

	facades.Register[*redsync.Redsync](redsync.New(pool))

	return nil
}
