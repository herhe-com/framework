package auth

import (
	"context"
	"fmt"
	"github.com/herhe-com/framework/facades"
	"strings"
	"time"
)

func CheckBlacklist(ctx context.Context, args ...any) bool {

	result, err := facades.Redis.Exists(ctx, KeyBlacklist(args...)).Result()

	return err == nil && result > 0
}

func Blacklist(ctx context.Context, value any, expires time.Duration, args ...any) bool {

	_, err := facades.Redis.Set(ctx, KeyBlacklist(args...), value, expires).Result()

	if err == nil {
		return true
	}

	return false
}

func KeyBlacklist(args ...any) string {

	keys := make([]string, 0)

	keys = append(keys, facades.Cfg.GetString("app.name"))
	keys = append(keys, "blacklist")

	for _, item := range args {
		keys = append(keys, fmt.Sprintf("%v", item))
	}

	return strings.Join(keys, ":")
}
