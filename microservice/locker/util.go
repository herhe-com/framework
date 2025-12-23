package locker

import (
	"github.com/herhe-com/framework/support/util"
)

func Keys(key string, keys ...any) string {

	items := make([]any, 0)

	items = append(items, "locker")
	items = append(items, key)

	for _, item := range keys {
		items = append(items, item)
	}

	return util.Keys(items...)

}
