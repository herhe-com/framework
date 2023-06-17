package locker

import (
	"fmt"
	"github.com/herhe-com/framework/facades"
	"strings"
)

func Keys(key string, keys ...any) string {

	items := []string{key}

	for _, item := range keys {
		items = append(items, fmt.Sprintf("%v", item))
	}

	return facades.Cfg.GetString("app.name") + ":locker:" + strings.Join(items, ":")
}
