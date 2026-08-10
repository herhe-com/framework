package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	contractauth "github.com/herhe-com/framework/contracts/auth"
	contractconfig "github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/facades"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Env(key string, defaultValue ...any) any {
	return f.Get(key, defaultValue...)
}

func (f fakeConfig) Add(name string, configuration map[string]any) {}

func (f fakeConfig) Set(key string, configuration any) {}

func (f fakeConfig) Get(key string, defaultValue ...any) any {
	if value, ok := f.values[key]; ok {
		return value
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (f fakeConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := f.values[key]; ok {
		return fmt.Sprint(value)
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

func (f fakeConfig) GetStrings(key string, defaultValue ...[]string) []string {
	return nil
}

func (f fakeConfig) GetMaps(key string, defaultValue ...map[string]any) map[string]any {
	return nil
}

func (f fakeConfig) GetInt(key string, defaultValue ...int) int {
	if value, ok := f.values[key].(int); ok {
		return value
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return 0
}

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 {
	if value, ok := f.values[key].(int64); ok {
		return value
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return 0
}

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool {
	return false
}

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

func TestNewJWTokenCreatesAccessOnlyToken(t *testing.T) {
	registerJWTConfigOnly(t)

	token, err := NewJWToken("user-1", 5, true, map[string]any{"role": "admin"})
	if err != nil {
		t.Fatalf("expected token to be created: %v", err)
	}

	var claims contractauth.Claims
	refresh, err := CheckJWToken(&claims, token)
	if err != nil {
		t.Fatalf("expected token to be valid: %v", err)
	}
	if refresh {
		t.Fatal("expected compatibility check to never auto-refresh")
	}
	if claims.Type != JWTTypeAccess {
		t.Fatalf("expected access token type, got %q", claims.Type)
	}
	if claims.Refresh {
		t.Fatal("expected legacy refresh flag to remain disabled")
	}
	if claims.Subject != "user-1" || claims.Issuer != "framework:api" {
		t.Fatalf("unexpected registered claims: %#v", claims.RegisteredClaims)
	}
}

func TestNewLoginJWTokenUsesConfiguredLifetimes(t *testing.T) {
	t.Run("refresh lifetime is days", func(t *testing.T) {
		registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}, map[string]any{
			"jwt.secret":           "test-secret",
			"jwt.lifetime":         15,
			"jwt.refresh.lifetime": 7,
		})

		pair, err := NewLoginJWToken("user-1", true, nil)
		if err != nil {
			t.Fatalf("expected login token pair to be created: %v", err)
		}

		var accessClaims contractauth.Claims
		if err := CheckAccessToken(&accessClaims, pair.AccessToken); err != nil {
			t.Fatalf("expected access token to be valid: %v", err)
		}
		var refreshClaims contractauth.Claims
		if err := checkRefreshToken(&refreshClaims, pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be valid: %v", err)
		}
		if accessClaims.Issuer != "framework:default" {
			t.Fatalf("expected configured default subject in issuer, got %q", accessClaims.Issuer)
		}
		if lifetime := accessClaims.ExpiresAt.Sub(accessClaims.IssuedAt.Time); lifetime != 15*time.Minute {
			t.Fatalf("expected 15-minute access lifetime, got %s", lifetime)
		}
		if lifetime := refreshClaims.ExpiresAt.Sub(refreshClaims.IssuedAt.Time); lifetime != 7*24*time.Hour {
			t.Fatalf("expected seven-day refresh lifetime, got %s", lifetime)
		}
	})

	t.Run("access only", func(t *testing.T) {
		registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}, map[string]any{
			"jwt.secret":           "test-secret",
			"jwt.sub":              "api",
			"jwt.lifetime":         10,
			"jwt.refresh.lifetime": 0,
		})

		pair, err := NewLoginJWToken("user-1", false, nil)
		if err != nil {
			t.Fatalf("expected access-only login token to be created: %v", err)
		}
		if pair.RefreshToken != "" || pair.RefreshLifetime != 0 {
			t.Fatalf("expected refresh fields to be empty, got %#v", pair)
		}
		var claims contractauth.Claims
		if err := CheckAccessToken(&claims, pair.AccessToken); err != nil {
			t.Fatalf("expected access token to be valid: %v", err)
		}
		if lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time); lifetime != 10*time.Minute {
			t.Fatalf("expected ten-minute access lifetime, got %s", lifetime)
		}
		if pair.IssuedAt != claims.IssuedAt.Unix() || pair.AccessLifetime != int64(10*time.Minute/time.Second) {
			t.Fatalf("expected access metadata to match claims, got %#v", pair)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}, map[string]any{
			"jwt.secret": "test-secret",
		})

		pair, err := NewLoginJWToken("user-1", true, nil)
		if err != nil {
			t.Fatalf("expected default login token pair to be created: %v", err)
		}
		var accessClaims contractauth.Claims
		if err := CheckAccessToken(&accessClaims, pair.AccessToken); err != nil {
			t.Fatalf("expected access token to be valid: %v", err)
		}
		var refreshClaims contractauth.Claims
		if err := checkRefreshToken(&refreshClaims, pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be valid: %v", err)
		}
		if lifetime := accessClaims.ExpiresAt.Sub(accessClaims.IssuedAt.Time); lifetime != defaultJWTAccessLifetimeMinutes*time.Minute {
			t.Fatalf("expected default access lifetime, got %s", lifetime)
		}
		if lifetime := refreshClaims.ExpiresAt.Sub(refreshClaims.IssuedAt.Time); lifetime != defaultJWTRefreshLifetimeDays*24*time.Hour {
			t.Fatalf("expected default refresh lifetime, got %s", lifetime)
		}
	})
}

func TestNewJWTokensCreatesIndependentTokenPair(t *testing.T) {
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
	}, map[string]any{
		"jwt.secret": "test-secret",
		"jwt.sub":    "api",
	})

	pair, err := NewJWTokens("user-1", 5, 60, map[string]any{"role": "admin"})
	if err != nil {
		t.Fatalf("expected token pair to be created: %v", err)
	}
	if pair.SessionID == "" {
		t.Fatalf("unexpected token pair metadata: %#v", pair)
	}

	var accessClaims contractauth.Claims
	if err := CheckAccessToken(&accessClaims, pair.AccessToken); err != nil {
		t.Fatalf("expected access token to be valid: %v", err)
	}
	var refreshClaims contractauth.Claims
	if err := checkRefreshToken(&refreshClaims, pair.RefreshToken); err != nil {
		t.Fatalf("expected refresh token to be valid: %v", err)
	}

	if accessClaims.Type != JWTTypeAccess || refreshClaims.Type != JWTTypeRefresh {
		t.Fatalf("unexpected token types: access=%q refresh=%q", accessClaims.Type, refreshClaims.Type)
	}
	if accessClaims.ID == refreshClaims.ID {
		t.Fatal("expected access and refresh tokens to have independent IDs")
	}
	if accessClaims.SessionID != refreshClaims.SessionID || accessClaims.SessionID != pair.SessionID {
		t.Fatalf("expected one shared session ID, access=%q refresh=%q pair=%q", accessClaims.SessionID, refreshClaims.SessionID, pair.SessionID)
	}
	if refreshClaims.AccessLifetime != int64(5*time.Minute/time.Second) {
		t.Fatalf("expected access lifetime claim %d, got %d", int64(5*time.Minute/time.Second), refreshClaims.AccessLifetime)
	}
	if pair.IssuedAt != accessClaims.IssuedAt.Unix() ||
		pair.AccessLifetime != int64(accessClaims.ExpiresAt.Sub(accessClaims.IssuedAt.Time)/time.Second) ||
		pair.RefreshLifetime != int64(refreshClaims.ExpiresAt.Sub(refreshClaims.IssuedAt.Time)/time.Second) {
		t.Fatal("expected response lifetime metadata to match signed claims")
	}
	if err := CheckAccessToken(&contractauth.Claims{}, pair.RefreshToken); !errors.Is(err, ErrInvalidJWTType) {
		t.Fatalf("expected refresh token to be rejected as access token, got %v", err)
	}
	if err := checkRefreshToken(&contractauth.Claims{}, pair.AccessToken); !errors.Is(err, ErrInvalidJWTType) {
		t.Fatalf("expected access token to be rejected as refresh token, got %v", err)
	}
}

func TestValidateAccessTokenUsesBloomBlacklist(t *testing.T) {
	redisUnavailable := errors.New("redis unavailable")
	tests := []struct {
		name        string
		blacklisted int64
		redisErr    error
		wantErr     error
	}{
		{name: "allowed"},
		{name: "blacklisted", blacklisted: 1, wantErr: ErrJWTBlacklisted},
		{name: "redis unavailable", redisErr: redisUnavailable, wantErr: ErrJWTBlacklistUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var args []any
			registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
				if cmd.FullName() != "bf.exists" {
					return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
				}
				if test.redisErr != nil {
					return test.redisErr
				}

				args = append([]any(nil), cmd.Args()...)
				cmd.(*redis.Cmd).SetVal(test.blacklisted)
				return nil
			}, map[string]any{
				"jwt.secret": "test-secret",
				"jwt.sub":    "api",
			})

			pair, err := NewJWTokens("user-1", 5, 60, nil)
			if err != nil {
				t.Fatalf("expected token pair to be created: %v", err)
			}
			var claims contractauth.Claims
			err = ValidateAccessToken(context.Background(), &claims, pair.AccessToken)
			if test.wantErr == nil && err != nil {
				t.Fatalf("expected access token to be allowed: %v", err)
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("expected error %v, got %v", test.wantErr, err)
			}
			if test.redisErr != nil {
				if !errors.Is(err, test.redisErr) {
					t.Fatalf("expected wrapped Redis error %v, got %v", test.redisErr, err)
				}
				return
			}
			if len(args) != 3 || fmt.Sprint(args[2]) != claims.ID {
				t.Fatalf("expected BF.EXISTS for access token ID %q, got %#v", claims.ID, args)
			}
		})
	}
}

func TestExpiredAccessTokenCannotBeRefreshed(t *testing.T) {
	registerJWTConfigOnly(t)

	now := time.Now().UTC().Truncate(time.Second)
	claims := contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "framework:api",
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		},
		Type:      JWTTypeAccess,
		SessionID: "session-1",
	}
	token, _, err := makeJWToken(claims)
	if err != nil {
		t.Fatalf("expected token to be signed: %v", err)
	}

	refresh, err := CheckJWToken(&contractauth.Claims{}, token)
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("expected expired-token error, got %v", err)
	}
	if refresh {
		t.Fatal("expected expired access token to never enter refresh flow")
	}
}

func TestRefreshJWTokensRejectsTokenBeforeNotBefore(t *testing.T) {
	registerJWTConfigOnly(t)

	now := time.Now().UTC().Truncate(time.Second)
	claims := contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "framework:api",
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Type:           JWTTypeRefresh,
		SessionID:      "session-1",
		AccessLifetime: int64(5 * time.Minute / time.Second),
	}
	token, _, err := makeJWToken(claims)
	if err != nil {
		t.Fatalf("expected refresh token to be signed: %v", err)
	}

	if _, err := RefreshJWTokens(context.Background(), token); !errors.Is(err, jwt.ErrTokenNotValidYet) {
		t.Fatalf("expected refresh token used before nbf to fail, got %v", err)
	}
}

func TestRefreshModeDefaultsToBlacklist(t *testing.T) {
	registerJWTConfigOnly(t)

	if mode := RefreshMode(); mode != RefreshModeBlacklist {
		t.Fatalf("expected default refresh mode %q, got %q", RefreshModeBlacklist, mode)
	}
}

func TestRefreshModeSupportsWhitelist(t *testing.T) {
	registerJWTConfigOnly(t, map[string]any{"jwt.refresh.mode": RefreshModeWhitelist})

	if mode := RefreshMode(); mode != RefreshModeWhitelist {
		t.Fatalf("expected refresh mode %q, got %q", RefreshModeWhitelist, mode)
	}
}

func TestWhitelistModeRegistersRefreshToken(t *testing.T) {
	var setArgs []any
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "set" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		setArgs = append([]any(nil), cmd.Args()...)
		cmd.(*redis.StatusCmd).SetVal("OK")
		return nil
	}, map[string]any{
		"jwt.secret":       "test-secret",
		"jwt.sub":          "api",
		"jwt.refresh.mode": RefreshModeWhitelist,
	})

	pair, err := NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected token pair to be created: %v", err)
	}
	var claims contractauth.Claims
	if err := checkRefreshToken(&claims, pair.RefreshToken); err != nil {
		t.Fatalf("expected refresh token to be valid: %v", err)
	}
	if len(setArgs) < 5 {
		t.Fatalf("expected whitelist SET EXAT arguments, got %#v", setArgs)
	}
	if key := fmt.Sprint(setArgs[1]); key != jwtRefreshKey("whitelist", claims.ID) {
		t.Fatalf("expected whitelist key %q, got %q", jwtRefreshKey("whitelist", claims.ID), key)
	}
	if !argumentsContain(setArgs, "exat") || !argumentsContain(setArgs, strconv.FormatInt(claims.ExpiresAt.Unix(), 10)) {
		t.Fatalf("expected exact refresh expiry in SET arguments, got %#v", setArgs)
	}
}

func TestRefreshJWTokensBlacklistReusesRedisGracePair(t *testing.T) {
	var capturedArgs []any
	var capturedScript string
	var candidates []contractauth.TokenPair
	var selected contractauth.TokenPair
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "eval" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		args := append([]any(nil), cmd.Args()...)
		candidate, err := candidatePairFromEvalArgs(args)
		if err != nil {
			return err
		}
		candidates = append(candidates, candidate)
		if len(candidates) == 1 {
			capturedArgs = args
			capturedScript = fmt.Sprint(args[1])
			selected = candidate
		}
		cmd.(*redis.Cmd).SetVal(tokenPairHashResult(selected, 5))
		return nil
	}, map[string]any{
		"jwt.secret":         "test-secret",
		"jwt.sub":            "api",
		"jwt.refresh.leeway": int64(5),
	})

	initial, err := NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected initial pair to be created: %v", err)
	}
	var oldClaims contractauth.Claims
	if err := checkRefreshToken(&oldClaims, initial.RefreshToken); err != nil {
		t.Fatalf("expected initial refresh token to be valid: %v", err)
	}

	first, err := RefreshJWTokens(context.Background(), initial.RefreshToken)
	if err != nil {
		t.Fatalf("expected first refresh to succeed: %v", err)
	}
	second, err := RefreshJWTokens(context.Background(), initial.RefreshToken)
	if err != nil {
		t.Fatalf("expected grace refresh to succeed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected identical pairs during grace window:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(candidates) != 2 || candidates[0].AccessToken == candidates[1].AccessToken {
		t.Fatalf("expected two different server candidates, got %#v", candidates)
	}
	if first.GraceLifetime != 5 {
		t.Fatalf("expected five-second grace window, got %#v", first)
	}
	if len(capturedArgs) < 9 {
		t.Fatalf("expected blacklist Lua arguments, got %#v", capturedArgs)
	}
	if key := fmt.Sprint(capturedArgs[3]); !strings.Contains(key, ":blacklist:api:refresh:") {
		t.Fatalf("expected dated refresh blacklist bucket, got %q", key)
	}
	if key := fmt.Sprint(capturedArgs[4]); key != jwtRefreshKey("grace", oldClaims.ID) {
		t.Fatalf("expected grace hash key %q, got %q", jwtRefreshKey("grace", oldClaims.ID), key)
	}
	if id := fmt.Sprint(capturedArgs[5]); id != oldClaims.ID {
		t.Fatalf("expected old refresh token ID %q, got %q", oldClaims.ID, id)
	}
	assertRefreshScript(t, capturedScript, "BF.ADD")
}

func TestRefreshJWTokensWhitelistRotatesExactKey(t *testing.T) {
	var evalArgs []any
	var evalScript string
	setCount := 0
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		switch cmd.FullName() {
		case "set":
			setCount++
			cmd.(*redis.StatusCmd).SetVal("OK")
		case "eval":
			evalArgs = append([]any(nil), cmd.Args()...)
			evalScript = fmt.Sprint(evalArgs[1])
			candidate, err := candidatePairFromEvalArgs(evalArgs)
			if err != nil {
				return err
			}
			cmd.(*redis.Cmd).SetVal(tokenPairHashResult(candidate, 5))
		default:
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}
		return nil
	}, map[string]any{
		"jwt.secret":         "test-secret",
		"jwt.sub":            "api",
		"jwt.refresh.mode":   RefreshModeWhitelist,
		"jwt.refresh.leeway": int64(5),
	})

	initial, err := NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected initial pair to be created: %v", err)
	}
	var oldClaims contractauth.Claims
	if err := checkRefreshToken(&oldClaims, initial.RefreshToken); err != nil {
		t.Fatalf("expected initial refresh token to be valid: %v", err)
	}

	rotated, err := RefreshJWTokens(context.Background(), initial.RefreshToken)
	if err != nil {
		t.Fatalf("expected whitelist refresh to succeed: %v", err)
	}
	var newClaims contractauth.Claims
	if err := checkRefreshToken(&newClaims, rotated.RefreshToken); err != nil {
		t.Fatalf("expected rotated refresh token to be valid: %v", err)
	}
	if setCount != 1 {
		t.Fatalf("expected only initial whitelist registration outside Lua, got %d SET commands", setCount)
	}
	if len(evalArgs) < 9 {
		t.Fatalf("expected whitelist Lua arguments, got %#v", evalArgs)
	}
	if key := fmt.Sprint(evalArgs[3]); key != jwtRefreshKey("whitelist", oldClaims.ID) {
		t.Fatalf("expected old whitelist key %q, got %q", jwtRefreshKey("whitelist", oldClaims.ID), key)
	}
	if key := fmt.Sprint(evalArgs[4]); key != jwtRefreshKey("whitelist", newClaims.ID) {
		t.Fatalf("expected new whitelist key %q, got %q", jwtRefreshKey("whitelist", newClaims.ID), key)
	}
	if key := fmt.Sprint(evalArgs[5]); key != jwtRefreshKey("grace", oldClaims.ID) {
		t.Fatalf("expected grace hash key %q, got %q", jwtRefreshKey("grace", oldClaims.ID), key)
	}
	assertRefreshScript(t, evalScript, "DEL")
}

func TestRefreshJWTokensRejectsConsumedTokenOutsideGrace(t *testing.T) {
	registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
		if cmd.FullName() != "eval" {
			return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
		}

		cmd.(*redis.Cmd).SetVal([]any{})
		return nil
	}, map[string]any{
		"jwt.secret": "test-secret",
		"jwt.sub":    "api",
	})

	initial, err := NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected initial pair to be created: %v", err)
	}
	if _, err := RefreshJWTokens(context.Background(), initial.RefreshToken); !errors.Is(err, ErrRefreshTokenUnavailable) {
		t.Fatalf("expected consumed-token error, got %v", err)
	}
}

func TestRefreshJWTokensRequiresRedis(t *testing.T) {
	registerJWTConfigOnly(t)

	initial, err := NewJWTokens("user-1", 5, 60, nil)
	if err != nil {
		t.Fatalf("expected blacklist issuance without Redis to succeed: %v", err)
	}
	if _, err := RefreshJWTokens(context.Background(), initial.RefreshToken); err == nil || !strings.Contains(err.Error(), "redis") {
		t.Fatalf("expected refresh to require Redis, got %v", err)
	}
}

func TestRevokeRefreshTokenUsesConfiguredMode(t *testing.T) {
	t.Run("blacklist", func(t *testing.T) {
		var evalArgs []any
		registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
			if cmd.FullName() != "eval" {
				return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
			}

			evalArgs = append([]any(nil), cmd.Args()...)
			cmd.(*redis.Cmd).SetVal(int64(1))
			return nil
		}, map[string]any{
			"jwt.secret": "test-secret",
			"jwt.sub":    "api",
		})

		pair, err := NewJWTokens("user-1", 5, 60, nil)
		if err != nil {
			t.Fatalf("expected pair to be created: %v", err)
		}
		var claims contractauth.Claims
		if err := checkRefreshToken(&claims, pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be valid: %v", err)
		}
		if err := RevokeRefreshToken(context.Background(), pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be revoked: %v", err)
		}
		if len(evalArgs) < 7 || !strings.Contains(fmt.Sprint(evalArgs[1]), "BF.ADD") {
			t.Fatalf("expected blacklist revoke Lua, got %#v", evalArgs)
		}
		if key := fmt.Sprint(evalArgs[3]); !strings.Contains(key, ":blacklist:api:refresh:") {
			t.Fatalf("expected issuer-isolated refresh blacklist key, got %q", key)
		}
		if key := fmt.Sprint(evalArgs[4]); key != jwtRefreshKey("grace", claims.ID) {
			t.Fatalf("expected grace key %q to be deleted, got %q", jwtRefreshKey("grace", claims.ID), key)
		}
	})

	t.Run("whitelist", func(t *testing.T) {
		var delArgs []any
		registerBlacklistTestServices(t, func(_ context.Context, cmd redis.Cmder) error {
			switch cmd.FullName() {
			case "set":
				cmd.(*redis.StatusCmd).SetVal("OK")
			case "del":
				delArgs = append([]any(nil), cmd.Args()...)
				cmd.(*redis.IntCmd).SetVal(2)
			default:
				return fmt.Errorf("unexpected Redis command: %s", cmd.FullName())
			}
			return nil
		}, map[string]any{
			"jwt.secret":       "test-secret",
			"jwt.sub":          "api",
			"jwt.refresh.mode": RefreshModeWhitelist,
		})

		pair, err := NewJWTokens("user-1", 5, 60, nil)
		if err != nil {
			t.Fatalf("expected pair to be created: %v", err)
		}
		var claims contractauth.Claims
		if err := checkRefreshToken(&claims, pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be valid: %v", err)
		}
		if err := RevokeRefreshToken(context.Background(), pair.RefreshToken); err != nil {
			t.Fatalf("expected refresh token to be revoked: %v", err)
		}
		if len(delArgs) != 3 {
			t.Fatalf("expected whitelist and grace keys to be deleted, got %#v", delArgs)
		}
		if fmt.Sprint(delArgs[1]) != jwtRefreshKey("whitelist", claims.ID) || fmt.Sprint(delArgs[2]) != jwtRefreshKey("grace", claims.ID) {
			t.Fatalf("unexpected whitelist revoke keys: %#v", delArgs)
		}
	})
}

func TestRefreshJWTokenCompatibilityEntryFailsExplicitly(t *testing.T) {
	if _, err := RefreshJWToken(context.Background(), &contractauth.Claims{}); !errors.Is(err, ErrRefreshJWTokenUnsupported) {
		t.Fatalf("expected legacy refresh API error, got %v", err)
	}
}

func registerJWTConfigOnly(t *testing.T, configs ...map[string]any) {
	t.Helper()

	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	values := map[string]any{
		"app.name":   "framework",
		"jwt.secret": "test-secret",
		"jwt.sub":    "api",
	}
	for _, config := range configs {
		for key, value := range config {
			values[key] = value
		}
	}
	facades.Register[contractconfig.Application](fakeConfig{values: values})

	t.Cleanup(func() {
		facades.SetContainer(original)
	})
}

func candidatePairFromEvalArgs(args []any) (contractauth.TokenPair, error) {
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
			AccessToken:     accessToken,
			RefreshToken:    fmt.Sprint(args[index+1]),
			IssuedAt:        issuedAt,
			AccessLifetime:  accessLifetime,
			RefreshLifetime: refreshLifetime,
			SessionID:       fmt.Sprint(args[index+5]),
		}, nil
	}

	return contractauth.TokenPair{}, errors.New("candidate token pair not found in Eval arguments")
}

func tokenPairHashResult(pair contractauth.TokenPair, graceLifetime int64) []any {
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

func assertRefreshScript(t *testing.T, script, consumeCommand string) {
	t.Helper()

	for _, fragment := range []string{
		"HGETALL",
		consumeCommand,
		"TIME",
		"HSET",
		"EXPIREAT",
		"access_token",
		"refresh_token",
		"issued_at",
		"access_lifetime",
		"refresh_lifetime",
		"grace_lifetime",
		"session_id",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected refresh Lua to contain %q", fragment)
		}
	}
	if strings.Index(script, "HGETALL") > strings.Index(script, consumeCommand) {
		t.Fatalf("expected grace hash lookup before consuming old refresh token")
	}
}

func argumentsContain(args []any, expected string) bool {
	for _, value := range args {
		if strings.EqualFold(fmt.Sprint(value), expected) {
			return true
		}
	}

	return false
}
