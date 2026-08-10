package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/facades"
)

const luaSetBloomBlacklist = `
	redis.call("BF.ADD", KEYS[1], ARGV[1])
	redis.call("EXPIREAT", KEYS[1], ARGV[2])
	return 1
`

func CheckBlacklist(ctx context.Context, args ...any) bool {
	result, err := CheckBlacklistByKey(ctx, KeyBlacklist(args...))
	return err == nil && result
}

// CheckBlacklistByKey checks whether an exact blacklist key exists.
func CheckBlacklistByKey(ctx context.Context, key string) (bool, error) {
	result, err := facades.Redis().Default().Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}

// CheckBloomBlacklist checks whether a value may exist in a Redis Bloom filter.
func CheckBloomBlacklist(ctx context.Context, key string, value any) (bool, error) {
	cache, ok := facades.OptionalRedis()
	if !ok {
		return false, fmt.Errorf("redis cannot be null")
	}

	return CheckBloomBlacklistWithRedis(cache, ctx, key, value)
}

// CheckBloomBlacklistWithRedis checks whether a value may exist in a Redis Bloom filter.
func CheckBloomBlacklistWithRedis(cache database.Redis, ctx context.Context, key string, value any) (bool, error) {
	result, err := cache.Default().Do(ctx, "BF.EXISTS", key, value).Bool()
	if err != nil {
		return false, err
	}

	return result, nil
}

func Blacklist(ctx context.Context, value any, expires time.Duration, args ...any) bool {
	cache := facades.Redis()
	return BlacklistWithRedis(cache, ctx, value, expires, args...)
}

func BlacklistWithRedis(cache database.Redis, ctx context.Context, value any, expires time.Duration, args ...any) bool {
	return SetBlacklistWithRedis(cache, ctx, value, expires, args...) == nil
}

// SetBlacklistWithRedis stores a blacklist entry with an expiry.
func SetBlacklistWithRedis(cache database.Redis, ctx context.Context, value any, expires time.Duration, args ...any) error {
	return cache.Default().Set(ctx, KeyBlacklist(args...), value, expires).Err()
}

// SetBloomBlacklistWithRedis adds a value to a Redis Bloom filter and expires the whole filter at expiresAt.
func SetBloomBlacklistWithRedis(cache database.Redis, ctx context.Context, key string, value any, expiresAt time.Time) error {
	return cache.Default().Eval(ctx, luaSetBloomBlacklist, []string{key}, value, expiresAt.Unix()).Err()
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
