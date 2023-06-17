package cache

import (
	"fmt"
	"github.com/herhe-com/framework/facades"
	"strings"
	"time"
)

func keys(args ...any) string {

	name := facades.Cfg.GetString("app.name")

	names := make([]string, 0)

	names = append(names, name)

	for _, item := range args {
		names = append(names, fmt.Sprintf("%v", item))
	}

	return strings.Join(names, ":")
}

func ttl() time.Duration {

	ttl := facades.Cfg.GetInt64("cache.ttl")

	return time.Duration(ttl)
}
