package locker

import (
	"github.com/bsm/redislock"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() (err error) {

	facades.Locker = redislock.New(facades.Redis)

	return nil
}
