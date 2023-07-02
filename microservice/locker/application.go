package locker

import (
	"errors"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() (err error) {

	if facades.Redis == nil {
		return errors.New("please initialize Redis first")
	}

	pool := goredis.NewPool(facades.Redis)

	facades.Locker = redsync.New(pool)

	return nil
}
