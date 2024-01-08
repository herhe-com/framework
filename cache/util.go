package cache

import (
	"fmt"
	"github.com/herhe-com/framework/facades"
	"strings"
	"time"
)

func keys(args ...any) string {

	name := facades.Cfg.GetString("server.name")

	names := make([]string, 0)

	names = append(names, name)

	for _, item := range args {
		names = append(names, fmt.Sprintf("%v", item))
	}

	return strings.Join(names, ":")
}

func ttl() time.Duration {

	t := facades.Cfg.GetInt64("cache.ttl")

	if t <= 0 {
		t = 120
	}

	return time.Minute * time.Duration(t)
}
