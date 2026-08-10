package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	contractauth "github.com/herhe-com/framework/contracts/auth"
	contractconfig "github.com/herhe-com/framework/contracts/config"
	contractdatabase "github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/facades"
)

type blacklistTestRedis struct {
	client *redis.Client
}

func (r blacklistTestRedis) Default() *redis.Client {
	return r.client
}

func (r blacklistTestRedis) Channel(string) (*redis.Client, error) {
	return r.client, nil
}

type blacklistTestHook struct {
	process func(context.Context, redis.Cmder) error
}

func (h blacklistTestHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h blacklistTestHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return h.process
}

func (h blacklistTestHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func registerBlacklistTestServices(t *testing.T, process func(context.Context, redis.Cmder) error, configs ...map[string]any) {
	t.Helper()

	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	values := map[string]any{
		"app.name": "framework",
	}
	for _, config := range configs {
		for key, value := range config {
			values[key] = value
		}
	}
	facades.Register[contractconfig.Application](fakeConfig{
		values: values,
	})

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	client.AddHook(blacklistTestHook{process: process})
	facades.Register[contractdatabase.Redis](blacklistTestRedis{client: client})

	t.Cleanup(func() {
		facades.SetContainer(original)
		if err := client.Close(); err != nil {
			t.Errorf("failed to close Redis client: %v", err)
		}
	})
}

func TestCheckBlacklistByKeyUsesExactKey(t *testing.T) {
	key := "framework:blacklist:jwt:token-1"
	var capturedKey string

	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "exists" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		capturedKey = fmt.Sprint(cmd.Args()[1])
		cmd.(*redis.IntCmd).SetVal(1)
		return nil
	})

	result, err := CheckBlacklistByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("expected blacklist lookup to succeed: %v", err)
	}
	if !result {
		t.Fatal("expected key to be blacklisted")
	}
	if capturedKey != key {
		t.Fatalf("expected Redis key %q, got %q", key, capturedKey)
	}
	if count := strings.Count(capturedKey, "framework:blacklist:"); count != 1 {
		t.Fatalf("expected blacklist prefix once, got %d in %q", count, capturedKey)
	}
}

func TestCheckBlacklistByKeyReturnsRedisError(t *testing.T) {
	expectedErr := errors.New("Redis unavailable")
	registerBlacklistTestServices(t, func(context.Context, redis.Cmder) error {
		return expectedErr
	})

	result, err := CheckBlacklistByKey(context.Background(), "framework:blacklist:jwt:token-1")
	if result {
		t.Fatal("expected failed lookup to return false")
	}
	if err != expectedErr {
		t.Fatalf("expected Redis error %v, got %v", expectedErr, err)
	}
}

func TestCheckBlacklistOfJwtUsesBloomFilter(t *testing.T) {
	var args []any
	expiresAt := time.Now().Add(time.Hour)
	claims := contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "bf.exists" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		args = append([]any(nil), cmd.Args()...)
		cmd.(*redis.Cmd).SetVal(true)
		return nil
	})
	expectedKey := KeyBlacklist("jwt", expiresAt.UTC().Format("20060102"))

	ctx := app.NewContext(0)
	ctx.Set(ContextOfClaims, claims)

	result, err := CheckBlacklistOfJwt(context.Background(), ctx)
	if err != nil {
		t.Fatalf("expected blacklist lookup to succeed: %v", err)
	}
	if !result {
		t.Fatal("expected token ID to be blacklisted")
	}
	if len(args) != 3 {
		t.Fatalf("expected BF.EXISTS arguments, got %#v", args)
	}
	if key := fmt.Sprint(args[1]); key != expectedKey {
		t.Fatalf("expected Bloom filter key %q, got %q", expectedKey, key)
	}
	if id := fmt.Sprint(args[2]); id != claims.ID {
		t.Fatalf("expected token ID %q, got %q", claims.ID, id)
	}
}

func TestCheckBlacklistOfJwtReturnsRedisError(t *testing.T) {
	expectedErr := errors.New("Redis unavailable")
	registerBlacklistTestServices(t, func(context.Context, redis.Cmder) error {
		return expectedErr
	})

	ctx := app.NewContext(0)
	ctx.Set(ContextOfClaims, contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	result, err := CheckBlacklistOfJwt(context.Background(), ctx)
	if result {
		t.Fatal("expected failed lookup to return false")
	}
	if err != expectedErr {
		t.Fatalf("expected Redis error %v, got %v", expectedErr, err)
	}
}

func TestBlacklistOfJwtValueUsesBloomFilterBucket(t *testing.T) {
	var evalArgs []any
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "eval" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		evalArgs = append([]any(nil), cmd.Args()...)
		cmd.(*redis.Cmd).SetVal(int64(1))
		return nil
	})

	expiresAt := time.Now().Add(time.Hour)
	claims := contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	bucketDate := expiresAt.UTC()
	expectedKey := KeyBlacklist("jwt", bucketDate.Format("20060102"))
	expectedExpiresAt := time.Date(bucketDate.Year(), bucketDate.Month(), bucketDate.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)

	ctx := app.NewContext(0)
	ctx.Set(ContextOfClaims, claims)

	result, err := BlacklistOfJwtValue(context.Background(), ctx)
	if err != nil {
		t.Fatalf("expected blacklist write to succeed: %v", err)
	}
	if !result {
		t.Fatal("expected blacklist write to return true")
	}
	if len(evalArgs) != 6 {
		t.Fatalf("expected atomic Bloom blacklist arguments, got %#v", evalArgs)
	}
	if script := fmt.Sprint(evalArgs[1]); !strings.Contains(script, "BF.ADD") || !strings.Contains(script, "EXPIREAT") {
		t.Fatalf("expected BF.ADD and EXPIREAT in one Lua script, got %q", script)
	}
	if key := fmt.Sprint(evalArgs[3]); key != expectedKey {
		t.Fatalf("expected Bloom filter key %q, got %q", expectedKey, key)
	}
	if id := fmt.Sprint(evalArgs[4]); id != claims.ID {
		t.Fatalf("expected token ID %q, got %q", claims.ID, id)
	}
	if expires := fmt.Sprint(evalArgs[5]); expires != fmt.Sprint(expectedExpiresAt.Unix()) {
		t.Fatalf("expected bucket expiry %d, got %s", expectedExpiresAt.Unix(), expires)
	}
}

func TestBlacklistOfJwtValueReturnsRedisError(t *testing.T) {
	expectedErr := errors.New("Redis unavailable")
	registerBlacklistTestServices(t, func(context.Context, redis.Cmder) error {
		return expectedErr
	})

	ctx := app.NewContext(0)
	ctx.Set(ContextOfClaims, contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	result, err := BlacklistOfJwtValue(context.Background(), ctx)
	if result {
		t.Fatal("expected failed blacklist write to return false")
	}
	if err != expectedErr {
		t.Fatalf("expected Redis error %v, got %v", expectedErr, err)
	}
}
