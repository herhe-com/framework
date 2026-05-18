package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/facades"
)

func CheckBlacklist(ctx context.Context, args ...any) bool {

	result, err := facades.Redis().Default().Exists(ctx, KeyBlacklist(args...)).Result()

	return err == nil && result > 0
}

func Blacklist(ctx context.Context, value any, expires time.Duration, args ...any) bool {
	cache := facades.Redis()
	return BlacklistWithRedis(cache, ctx, value, expires, args...)
}

func BlacklistWithRedis(cache database.Redis, ctx context.Context, value any, expires time.Duration, args ...any) bool {
	err := cache.Default().Set(ctx, KeyBlacklist(args...), value, expires).Err()
	return err == nil
}

func KeyBlacklist(args ...any) string {

	keys := make([]string, 0)

	keys = append(keys, facades.Config().GetString("app.name"))
	keys = append(keys, "blacklist")

	for _, item := range args {
		keys = append(keys, fmt.Sprintf("%v", item))
	}

	return strings.Join(keys, ":")
}
