package cache

import (
	"time"

	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/support/util"
)

func Keys(args ...any) string {

	return util.Keys(args...)
}

func TTL() time.Duration {

	t := facades.Cfg.GetInt64("cache.TTL")

	if t <= 0 {
		t = 120
	}

	return time.Minute * time.Duration(t)
}
