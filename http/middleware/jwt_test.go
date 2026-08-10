package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/herhe-com/framework/auth"
	frameworkconfig "github.com/herhe-com/framework/config"
	contractauth "github.com/herhe-com/framework/contracts/auth"
	contractdatabase "github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/facades"
)

type jwtMiddlewareRedis struct {
	client *redis.Client
}

func (r jwtMiddlewareRedis) Default() *redis.Client {
	return r.client
}

func (r jwtMiddlewareRedis) Channel(string) (*redis.Client, error) {
	return r.client, nil
}

type jwtMiddlewareRedisHook struct {
	process func(context.Context, redis.Cmder) error
}

func (h jwtMiddlewareRedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h jwtMiddlewareRedisHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return h.process
}

func (h jwtMiddlewareRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestJwtRefreshesWhenAccessTokenIsInvalid(t *testing.T) {
	registerJWTMiddlewareServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "eval" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		candidate, err := jwtMiddlewareCandidatePair(cmd.Args())
		if err != nil {
			return err
		}
		cmd.(*redis.Cmd).SetVal(jwtMiddlewareHashResult(candidate, 5))
		return nil
	})

	initial, err := auth.NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected initial token pair to be created: %v", err)
	}
	requestCtx := app.NewContext(0)
	requestCtx.Request.Header.Set(auth.JwtOfAuthorization, "invalid-access-token")
	requestCtx.Request.Header.Set(auth.JwtOfRefreshToken, initial.RefreshToken)
	Jwt()(context.Background(), requestCtx)

	if id := auth.ID(requestCtx); id != "user-1" {
		t.Fatalf("expected refreshed request identity user-1, got %q", id)
	}
	claims := auth.Claims(requestCtx)
	if claims == nil || claims.Type != auth.JWTTypeAccess {
		t.Fatalf("expected refreshed access claims, got %#v", claims)
	}

	var pair contractauth.TokenPair
	header := requestCtx.Response.Header.Peek(auth.JwtOfTokenPair)
	if err := json.Unmarshal(header, &pair); err != nil {
		t.Fatalf("expected Token-Pair response header: %v", err)
	}
	if strings.Contains(string(header), "token_type") {
		t.Fatalf("expected Token-Pair header without token_type, got %s", header)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" || pair.RefreshToken == initial.RefreshToken {
		t.Fatalf("expected rotated token pair, got %#v", pair)
	}
	if pair.IssuedAt <= 0 || pair.AccessLifetime != int64(5*time.Minute/time.Second) ||
		pair.RefreshLifetime != int64(time.Hour/time.Second) || pair.GraceLifetime != 5 {
		t.Fatalf("expected second-based token lifetime metadata, got %#v", pair)
	}
	if cacheControl := requestCtx.Response.Header.Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("expected refreshed token response not to be cached, got %q", cacheControl)
	}
}

func TestJwtRejectsRefreshTokenWhenAccessTokenIsValid(t *testing.T) {
	registerJWTMiddlewareServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "bf.exists" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		cmd.(*redis.Cmd).SetVal(int64(0))
		return nil
	})

	pair, err := auth.NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected token pair to be created: %v", err)
	}

	requestCtx := app.NewContext(0)
	requestCtx.Request.Header.Set(auth.JwtOfAuthorization, pair.AccessToken)
	requestCtx.Request.Header.Set(auth.JwtOfRefreshToken, pair.RefreshToken)
	Jwt()(context.Background(), requestCtx)

	if id := auth.ID(requestCtx); id != "" {
		t.Fatalf("expected request not to establish an authenticated context, got %q", id)
	}
	if header := requestCtx.Response.Header.Peek(auth.JwtOfTokenPair); len(header) != 0 {
		t.Fatalf("expected refresh token not to be consumed, got %s", header)
	}
	if body := string(requestCtx.Response.Body()); !strings.Contains(body, "Unauthorized") {
		t.Fatalf("expected unauthorized response, got %q", body)
	}
}

func TestJwtRejectsConfiguredCallbackError(t *testing.T) {
	registerJWTMiddlewareServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "bf.exists" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		cmd.(*redis.Cmd).SetVal(int64(0))
		return nil
	})

	accessToken, err := auth.NewJWToken("user-1", 5, false, nil)
	if err != nil {
		t.Fatalf("expected access token to be created: %v", err)
	}
	called := false
	facades.Config().Set("auth.callback.jwt", func(_ context.Context, ctx *app.RequestContext) error {
		called = true
		if auth.ID(ctx) != "user-1" {
			t.Fatal("expected callback to receive JWT request context")
		}

		return errors.New("JWT context is not allowed")
	})

	requestCtx := app.NewContext(0)
	requestCtx.Request.Header.Set(auth.JwtOfAuthorization, accessToken)
	Jwt()(context.Background(), requestCtx)

	if !called {
		t.Fatal("expected configured JWT callback to be called")
	}
	if body := string(requestCtx.Response.Body()); !strings.Contains(body, "Unauthorized") {
		t.Fatalf("expected unauthorized response, got %q", body)
	}
}

func TestJwtRejectsRefreshTokenBeforeNotBefore(t *testing.T) {
	registerJWTMiddlewareServices(t, func(_ context.Context, cmd redis.Cmder) error {
		return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
	})

	now := time.Now().UTC().Truncate(time.Second)
	refreshToken, err := auth.MakeJWToken(contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "framework:api",
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Type:           auth.JWTTypeRefresh,
		SessionID:      "session-1",
		AccessLifetime: int64(5 * time.Minute / time.Second),
	})
	if err != nil {
		t.Fatalf("expected refresh token to be signed: %v", err)
	}

	requestCtx := app.NewContext(0)
	requestCtx.Request.Header.Set(auth.JwtOfAuthorization, "invalid-access-token")
	requestCtx.Request.Header.Set(auth.JwtOfRefreshToken, refreshToken)
	Jwt()(context.Background(), requestCtx)

	if id := auth.ID(requestCtx); id != "" {
		t.Fatalf("expected request to remain unauthenticated, got %q", id)
	}
	if body := string(requestCtx.Response.Body()); !strings.Contains(body, "Unauthorized") {
		t.Fatalf("expected direct unauthorized response, got %q", body)
	}
}

func registerJWTMiddlewareServices(t *testing.T, process func(context.Context, redis.Cmder) error) {
	t.Helper()

	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	t.Setenv("HH_CFG_PROVIDER", "")
	t.Setenv("HH_CFG_ENDPOINT", "")
	t.Setenv("HH_CFG_PATH", "")
	t.Setenv("HH_CFG_WATCH", "")
	t.Setenv("HH_CFG_SECRET", "")
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("expected config application to initialize: %v", err)
	}
	facades.Config().Set("app.name", "framework")
	facades.Config().Set("jwt.secret", "test-secret")
	facades.Config().Set("jwt.sub", "api")
	facades.Config().Set("jwt.refresh.leeway", int64(5))

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	client.AddHook(jwtMiddlewareRedisHook{process: process})
	facades.Register[contractdatabase.Redis](jwtMiddlewareRedis{client: client})

	t.Cleanup(func() {
		facades.SetContainer(original)
		if err := client.Close(); err != nil {
			t.Errorf("failed to close Redis client: %v", err)
		}
	})
}

func jwtMiddlewareCandidatePair(args []any) (contractauth.TokenPair, error) {
	for index, value := range args {
		accessToken := fmt.Sprint(value)
		if index+5 >= len(args) || strings.Count(accessToken, ".") != 2 || strings.Count(fmt.Sprint(args[index+1]), ".") != 2 {
			continue
		}

		issuedAt, err := strconv.ParseInt(fmt.Sprint(args[index+2]), 10, 64)
		if err != nil {
			return contractauth.TokenPair{}, err
		}
		accessLifetime, err := strconv.ParseInt(fmt.Sprint(args[index+3]), 10, 64)
		if err != nil {
			return contractauth.TokenPair{}, err
		}
		refreshLifetime, err := strconv.ParseInt(fmt.Sprint(args[index+4]), 10, 64)
		if err != nil {
			return contractauth.TokenPair{}, err
		}

		return contractauth.TokenPair{
			SessionID:       fmt.Sprint(args[index+5]),
			AccessToken:     accessToken,
			RefreshToken:    fmt.Sprint(args[index+1]),
			IssuedAt:        issuedAt,
			AccessLifetime:  accessLifetime,
			RefreshLifetime: refreshLifetime,
		}, nil
	}

	return contractauth.TokenPair{}, fmt.Errorf("candidate token pair not found")
}

func jwtMiddlewareHashResult(pair contractauth.TokenPair, graceLifetime int64) []any {
	return []any{
		"access_token", pair.AccessToken,
		"refresh_token", pair.RefreshToken,
		"issued_at", strconv.FormatInt(pair.IssuedAt, 10),
		"access_lifetime", strconv.FormatInt(pair.AccessLifetime, 10),
		"refresh_lifetime", strconv.FormatInt(pair.RefreshLifetime, 10),
		"grace_lifetime", strconv.FormatInt(graceLifetime, 10),
		"session_id", pair.SessionID,
	}
}
