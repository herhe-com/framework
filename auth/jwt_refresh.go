package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	contractauth "github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/facades"
)

const (
	defaultJWTAccessLifetimeMinutes = 720
	defaultJWTRefreshLifetimeDays   = 30

	luaRefreshWithBlacklist = `
		local cached = redis.call("HGETALL", KEYS[2])
		if #cached > 0 then
			return cached
		end

		if redis.call("BF.ADD", KEYS[1], ARGV[1]) == 0 then
			return {}
		end

		redis.call("EXPIREAT", KEYS[1], ARGV[2])

		local leeway = tonumber(ARGV[3])
		local grace_expires_at = tonumber(redis.call("TIME")[1]) + leeway
		local pair = {
			"access_token", ARGV[4],
			"refresh_token", ARGV[5],
			"issued_at", ARGV[6],
			"access_lifetime", ARGV[7],
			"refresh_lifetime", ARGV[8],
			"grace_lifetime", tostring(leeway),
			"session_id", ARGV[9]
		}

		if leeway > 0 then
			redis.call("HSET", KEYS[2], unpack(pair))
			redis.call("EXPIREAT", KEYS[2], grace_expires_at)
		end

		return pair
	`

	luaRefreshWithWhitelist = `
		local cached = redis.call("HGETALL", KEYS[3])
		if #cached > 0 then
			return cached
		end

		if redis.call("DEL", KEYS[1]) == 0 then
			return {}
		end

		redis.call("SET", KEYS[2], "1", "EXAT", ARGV[1])

		local leeway = tonumber(ARGV[2])
		local grace_expires_at = tonumber(redis.call("TIME")[1]) + leeway
		local pair = {
			"access_token", ARGV[3],
			"refresh_token", ARGV[4],
			"issued_at", ARGV[5],
			"access_lifetime", ARGV[6],
			"refresh_lifetime", ARGV[7],
			"grace_lifetime", tostring(leeway),
			"session_id", ARGV[8]
		}

		if leeway > 0 then
			redis.call("HSET", KEYS[3], unpack(pair))
			redis.call("EXPIREAT", KEYS[3], grace_expires_at)
		end

		return pair
	`

	luaRevokeRefreshWithBlacklist = `
		redis.call("BF.ADD", KEYS[1], ARGV[1])
		redis.call("EXPIREAT", KEYS[1], ARGV[2])
		redis.call("DEL", KEYS[2])
		return 1
	`
)

var (
	// ErrRefreshTokenUnavailable is returned when a refresh token was already used or revoked.
	ErrRefreshTokenUnavailable = errors.New("refresh token is no longer available")

	// ErrRefreshServiceUnavailable is returned when refresh-token state cannot be read or changed.
	ErrRefreshServiceUnavailable = errors.New("refresh service is unavailable")
)

// NewLoginJWToken creates login token metadata using the configured JWT lifetimes.
// jwt.lifetime is expressed in minutes and jwt.refresh.lifetime is expressed in days.
func NewLoginJWToken(userID string, refresh bool, ext map[string]any) (contractauth.TokenPair, error) {
	accessLifetime := facades.Config().GetInt("jwt.lifetime", defaultJWTAccessLifetimeMinutes)
	if refresh {
		refreshLifetime := facades.Config().GetInt("jwt.refresh.lifetime", defaultJWTRefreshLifetimeDays)
		if refreshLifetime <= 0 {
			return contractauth.TokenPair{}, errors.New("refresh lifetime must be greater than zero")
		}
		if refreshLifetime > int(MaxRefreshLifetime/(24*time.Hour)) {
			return contractauth.TokenPair{}, fmt.Errorf("refresh lifetime cannot exceed %d days", int(MaxRefreshLifetime/(24*time.Hour)))
		}

		return NewJWTokens(userID, accessLifetime, refreshLifetime*24*60, ext)
	}

	accessToken, err := NewJWToken(userID, accessLifetime, false, ext)
	if err != nil {
		return contractauth.TokenPair{}, err
	}

	var claims contractauth.Claims
	if err := CheckAccessToken(&claims, accessToken); err != nil {
		return contractauth.TokenPair{}, err
	}

	return contractauth.TokenPair{
		SessionID:      claims.SessionID,
		AccessToken:    accessToken,
		IssuedAt:       claims.IssuedAt.Unix(),
		AccessLifetime: int64(claims.ExpiresAt.Sub(claims.IssuedAt.Time) / time.Second),
	}, nil
}

// NewJWTokens creates an independent access and refresh token pair.
// Both lifetime arguments are expressed in minutes.
func NewJWTokens(userID string, accessLifetime, refreshLifetime int, ext map[string]any) (contractauth.TokenPair, error) {
	if accessLifetime <= 0 {
		return contractauth.TokenPair{}, errors.New("access lifetime must be greater than zero")
	}
	if refreshLifetime <= 0 {
		return contractauth.TokenPair{}, errors.New("refresh lifetime must be greater than zero")
	}
	if refreshLifetime > int(MaxRefreshLifetime/time.Minute) {
		return contractauth.TokenPair{}, fmt.Errorf("refresh lifetime cannot exceed %d minutes", int(MaxRefreshLifetime/time.Minute))
	}

	now := time.Now().UTC().Truncate(time.Second)
	issuer := jwtIssuer()
	pair, refreshClaims, err := issueTokenPair(
		userID,
		issuer,
		id(now, issuer, userID),
		now,
		time.Duration(accessLifetime)*time.Minute,
		time.Duration(refreshLifetime)*time.Minute,
		ext,
	)
	if err != nil {
		return contractauth.TokenPair{}, err
	}

	if RefreshMode() == RefreshModeWhitelist {
		if err := registerRefreshToken(context.Background(), &refreshClaims); err != nil {
			return contractauth.TokenPair{}, err
		}
	}

	return pair, nil
}

// RefreshJWTokens atomically consumes a refresh token and rotates the token pair.
// Repeated use of the same refresh token during the configured grace window returns the same pair.
func RefreshJWTokens(ctx context.Context, refreshToken string, leeways ...int64) (contractauth.TokenPair, error) {
	var oldClaims contractauth.Claims
	if err := checkRefreshToken(&oldClaims, refreshToken); err != nil {
		return contractauth.TokenPair{}, err
	}

	cache, ok := facades.OptionalRedis()
	if !ok {
		return contractauth.TokenPair{}, fmt.Errorf("%w: redis cannot be null", ErrRefreshServiceUnavailable)
	}

	refreshLifetime, err := refreshTokenLifetime(&oldClaims)
	if err != nil {
		return contractauth.TokenPair{}, err
	}
	accessLifetime := time.Duration(oldClaims.AccessLifetime) * time.Second
	now := time.Now().UTC().Truncate(time.Second)
	candidate, newRefreshClaims, err := issueTokenPair(
		oldClaims.Subject,
		oldClaims.Issuer,
		oldClaims.SessionID,
		now,
		accessLifetime,
		refreshLifetime,
		oldClaims.Ext,
	)
	if err != nil {
		return contractauth.TokenPair{}, err
	}

	leeway := refreshLeeway(leeways...)
	var pair contractauth.TokenPair
	if RefreshMode() == RefreshModeWhitelist {
		pair, err = refreshWithWhitelist(cache, ctx, &oldClaims, &newRefreshClaims, candidate, leeway)
	} else {
		pair, err = refreshWithBlacklist(cache, ctx, &oldClaims, candidate, leeway)
	}
	if err == nil || errors.Is(err, ErrRefreshTokenUnavailable) {
		return pair, err
	}

	return contractauth.TokenPair{}, fmt.Errorf("%w: %w", ErrRefreshServiceUnavailable, err)
}

// RevokeRefreshToken prevents further use of a signed refresh token.
func RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	var claims contractauth.Claims
	if err := checkRefreshToken(&claims, refreshToken); err != nil {
		return err
	}

	cache, ok := facades.OptionalRedis()
	if !ok {
		return errors.New("redis cannot be null")
	}

	graceKey := jwtRefreshKey("grace", claims.ID)
	if RefreshMode() == RefreshModeWhitelist {
		return cache.Default().Del(ctx, jwtRefreshKey("whitelist", claims.ID), graceKey).Err()
	}

	key, expiresAt := jwtBlacklistDateBucket(claims.ExpiresAt.Time, "jwt", "refresh")
	return cache.Default().Eval(ctx, luaRevokeRefreshWithBlacklist, []string{
		key,
		graceKey,
	}, claims.ID, expiresAt.Unix()).Err()
}

func issueTokenPair(
	subject string,
	issuer string,
	sessionID string,
	issuedAt time.Time,
	accessLifetime time.Duration,
	refreshLifetime time.Duration,
	ext map[string]any,
) (contractauth.TokenPair, contractauth.Claims, error) {
	if subject == "" {
		return contractauth.TokenPair{}, contractauth.Claims{}, errors.New("subject cannot be empty")
	}
	if issuer == "" {
		return contractauth.TokenPair{}, contractauth.Claims{}, errors.New("issuer cannot be empty")
	}
	if sessionID == "" {
		return contractauth.TokenPair{}, contractauth.Claims{}, errors.New("session ID cannot be empty")
	}
	if accessLifetime <= 0 || refreshLifetime <= 0 || refreshLifetime > MaxRefreshLifetime {
		return contractauth.TokenPair{}, contractauth.Claims{}, errors.New("invalid token lifetime")
	}

	issuedAt = issuedAt.UTC().Truncate(time.Second)
	registered := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
	}

	accessClaims := contractauth.Claims{
		RegisteredClaims: registered,
		Type:             JWTTypeAccess,
		SessionID:        sessionID,
		Ext:              ext,
	}
	accessClaims.ExpiresAt = jwt.NewNumericDate(issuedAt.Add(accessLifetime))
	accessToken, signedAccessClaims, err := makeJWToken(accessClaims)
	if err != nil {
		return contractauth.TokenPair{}, contractauth.Claims{}, err
	}

	refreshClaims := contractauth.Claims{
		RegisteredClaims: registered,
		Type:             JWTTypeRefresh,
		SessionID:        sessionID,
		AccessLifetime:   int64(accessLifetime / time.Second),
		Ext:              ext,
	}
	refreshClaims.ExpiresAt = jwt.NewNumericDate(issuedAt.Add(refreshLifetime))
	refreshToken, signedRefreshClaims, err := makeJWToken(refreshClaims)
	if err != nil {
		return contractauth.TokenPair{}, contractauth.Claims{}, err
	}

	return contractauth.TokenPair{
		SessionID:       sessionID,
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		IssuedAt:        signedAccessClaims.IssuedAt.Unix(),
		AccessLifetime:  int64(signedAccessClaims.ExpiresAt.Sub(signedAccessClaims.IssuedAt.Time) / time.Second),
		RefreshLifetime: int64(signedRefreshClaims.ExpiresAt.Sub(signedRefreshClaims.IssuedAt.Time) / time.Second),
	}, signedRefreshClaims, nil
}

func checkRefreshToken(claims *contractauth.Claims, token string) error {
	if err := parseJWToken(claims, token); err != nil {
		return err
	}
	if claims.Type != JWTTypeRefresh {
		return fmt.Errorf("%w: expected %s", ErrInvalidJWTType, JWTTypeRefresh)
	}
	if claims.SessionID == "" || claims.AccessLifetime <= 0 {
		return errors.New("refresh token claims are incomplete")
	}

	_, err := refreshTokenLifetime(claims)
	return err
}

func refreshWithBlacklist(
	cache database.Redis,
	ctx context.Context,
	oldClaims *contractauth.Claims,
	candidate contractauth.TokenPair,
	leeway int64,
) (contractauth.TokenPair, error) {
	key, expiresAt := jwtBlacklistDateBucket(oldClaims.ExpiresAt.Time, "jwt", "refresh")
	args := []any{oldClaims.ID, expiresAt.Unix(), leeway}
	args = append(args, tokenPairScriptArguments(candidate)...)
	result, err := cache.Default().Eval(ctx, luaRefreshWithBlacklist, []string{
		key,
		jwtRefreshKey("grace", oldClaims.ID),
	}, args...).Slice()
	if err != nil {
		return contractauth.TokenPair{}, err
	}

	return tokenPairFromHash(result)
}

func refreshWithWhitelist(
	cache database.Redis,
	ctx context.Context,
	oldClaims *contractauth.Claims,
	newClaims *contractauth.Claims,
	candidate contractauth.TokenPair,
	leeway int64,
) (contractauth.TokenPair, error) {
	args := []any{newClaims.ExpiresAt.Unix(), leeway}
	args = append(args, tokenPairScriptArguments(candidate)...)
	result, err := cache.Default().Eval(ctx, luaRefreshWithWhitelist, []string{
		jwtRefreshKey("whitelist", oldClaims.ID),
		jwtRefreshKey("whitelist", newClaims.ID),
		jwtRefreshKey("grace", oldClaims.ID),
	}, args...).Slice()
	if err != nil {
		return contractauth.TokenPair{}, err
	}

	return tokenPairFromHash(result)
}

func registerRefreshToken(ctx context.Context, claims *contractauth.Claims) error {
	if claims == nil || claims.Type != JWTTypeRefresh || claims.ID == "" || claims.ExpiresAt == nil {
		return errors.New("invalid refresh token claims")
	}

	cache, ok := facades.OptionalRedis()
	if !ok {
		return errors.New("redis cannot be null")
	}

	return cache.Default().SetArgs(ctx, jwtRefreshKey("whitelist", claims.ID), "1", redis.SetArgs{
		ExpireAt: claims.ExpiresAt.Time,
	}).Err()
}

func refreshTokenLifetime(claims *contractauth.Claims) (time.Duration, error) {
	if claims == nil || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return 0, errors.New("invalid refresh token lifetime")
	}

	lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	if lifetime <= 0 || lifetime > MaxRefreshLifetime {
		return 0, errors.New("invalid refresh token lifetime")
	}

	return lifetime, nil
}

func refreshLeeway(leeways ...int64) int64 {
	leeway := facades.Config().GetInt64("jwt.refresh.leeway", facades.Config().GetInt64("jwt.leeway", 3))
	if len(leeways) > 0 {
		leeway = leeways[0]
	}
	if leeway < 0 {
		return 0
	}

	return leeway
}

func tokenPairScriptArguments(pair contractauth.TokenPair) []any {
	return []any{
		pair.AccessToken,
		pair.RefreshToken,
		pair.IssuedAt,
		pair.AccessLifetime,
		pair.RefreshLifetime,
		pair.SessionID,
	}
}

func tokenPairFromHash(result []any) (contractauth.TokenPair, error) {
	if len(result) == 0 {
		return contractauth.TokenPair{}, ErrRefreshTokenUnavailable
	}
	if len(result)%2 != 0 {
		return contractauth.TokenPair{}, errors.New("invalid refresh grace data")
	}

	values := make(map[string]string, len(result)/2)
	for index := 0; index < len(result); index += 2 {
		values[redisString(result[index])] = redisString(result[index+1])
	}

	pair := contractauth.TokenPair{
		AccessToken:  values["access_token"],
		RefreshToken: values["refresh_token"],
		SessionID:    values["session_id"],
	}
	var err error
	if pair.IssuedAt, err = parseTokenPairValue(values, "issued_at", false); err != nil {
		return contractauth.TokenPair{}, err
	}
	if pair.AccessLifetime, err = parseTokenPairValue(values, "access_lifetime", false); err != nil {
		return contractauth.TokenPair{}, err
	}
	if pair.RefreshLifetime, err = parseTokenPairValue(values, "refresh_lifetime", false); err != nil {
		return contractauth.TokenPair{}, err
	}
	if pair.GraceLifetime, err = parseTokenPairValue(values, "grace_lifetime", true); err != nil {
		return contractauth.TokenPair{}, err
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" || pair.SessionID == "" {
		return contractauth.TokenPair{}, errors.New("refresh grace data is incomplete")
	}

	return pair, nil
}

func parseTokenPairValue(values map[string]string, key string, allowZero bool) (int64, error) {
	value, err := strconv.ParseInt(values[key], 10, 64)
	if err != nil || value < 0 || (!allowZero && value == 0) {
		return 0, fmt.Errorf("invalid refresh grace field %s", key)
	}

	return value, nil
}

func redisString(value any) string {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}

	return fmt.Sprint(value)
}

func jwtRefreshKey(kind, id string) string {
	return strings.Join([]string{
		facades.Config().GetString("app.name"),
		"jwt",
		"refresh",
		kind,
		id,
	}, ":")
}
