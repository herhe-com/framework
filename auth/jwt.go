package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/dromara/dongle"
	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	contractauth "github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
)

const (
	jwtBlacklistBucketLayout = "20060102"

	// JWTTypeAccess identifies an access token.
	JWTTypeAccess = "access"

	// JWTTypeRefresh identifies a refresh token.
	JWTTypeRefresh = "refresh"

	// RefreshModeBlacklist only records refresh-token IDs after they are used or revoked.
	RefreshModeBlacklist = "blacklist"

	// RefreshModeWhitelist records every refresh-token ID when it is issued.
	RefreshModeWhitelist = "whitelist"

	// MaxRefreshLifetime is the maximum refresh-token lifetime.
	MaxRefreshLifetime = 30 * 24 * time.Hour

	// MaxRefreshLifetimeSeconds is MaxRefreshLifetime in seconds.
	MaxRefreshLifetimeSeconds = 86400 * 30
)

var (
	// ErrInvalidJWTType is returned when a refresh token is used as an access token or vice versa.
	ErrInvalidJWTType = errors.New("invalid JWT token type")

	// ErrJWTBlacklisted is returned when an access token has been revoked.
	ErrJWTBlacklisted = errors.New("JWT is blacklisted")

	// ErrJWTBlacklistUnavailable is returned when the access-token blacklist cannot be checked.
	ErrJWTBlacklistUnavailable = errors.New("JWT blacklist is unavailable")

	// ErrRefreshJWTokenUnsupported directs callers to the signed refresh-token flow.
	ErrRefreshJWTokenUnsupported = errors.New("RefreshJWToken is no longer supported; use RefreshJWTokens with a refresh token")
)

// RefreshMode returns the configured refresh-token storage mode.
func RefreshMode() string {
	mode := strings.ToLower(strings.TrimSpace(facades.Config().GetString("jwt.refresh.mode", RefreshModeBlacklist)))
	if mode == RefreshModeWhitelist {
		return RefreshModeWhitelist
	}

	return RefreshModeBlacklist
}

// NewJWToken creates an access token. The refresh argument is kept for source compatibility and is ignored.
// Use NewJWTokens when a refresh token is required.
func NewJWToken(userID string, lifetime int, _ bool, ext map[string]any) (string, error) {
	if lifetime <= 0 {
		return "", errors.New("lifetime must be greater than zero")
	}

	now := time.Now().UTC()
	issuer := jwtIssuer()
	claims := contractauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(lifetime) * time.Minute)),
		},
		Type:      JWTTypeAccess,
		SessionID: id(now, issuer, userID),
		Ext:       ext,
	}

	return MakeJWToken(claims)
}

// BlacklistOfJwtName returns the current access token's dated Bloom-filter key.
func BlacklistOfJwtName(ctx *app.RequestContext) string {
	key, _, _ := jwtBlacklistBucket(Claims(ctx))
	return key
}

// CheckBlacklistOfJwt checks whether the current access-token ID may exist in its blacklist bucket.
func CheckBlacklistOfJwt(ctx context.Context, requestCtx *app.RequestContext) (bool, error) {
	return checkJWTBlacklist(ctx, Claims(requestCtx))
}

func checkJWTBlacklist(ctx context.Context, claims *contractauth.Claims) (bool, error) {
	key, _, err := jwtBlacklistBucket(claims)
	if err != nil {
		return false, err
	}

	return CheckBloomBlacklist(ctx, key, claims.ID)
}

// BlacklistOfJwtValue revokes the current access token until its expiration date.
func BlacklistOfJwtValue(ctx context.Context, requestCtx *app.RequestContext) (bool, error) {
	claims := Claims(requestCtx)
	key, expiresAt, err := jwtBlacklistBucket(claims)
	if err != nil {
		return false, err
	}

	cache, ok := facades.OptionalRedis()
	if !ok {
		return false, errors.New("redis cannot be null")
	}

	if claims.ExpiresAt.Time.After(time.Now()) {
		if err := SetBloomBlacklistWithRedis(cache, ctx, key, claims.ID, expiresAt); err != nil {
			return false, err
		}
	}

	return true, nil
}

func jwtBlacklistBucket(claims *contractauth.Claims) (string, time.Time, error) {
	if claims == nil {
		return "", time.Time{}, errors.New("claims cannot be null")
	}
	if claims.ExpiresAt == nil {
		return "", time.Time{}, errors.New("expires_at cannot be null")
	}
	if claims.ID == "" {
		return "", time.Time{}, errors.New("id cannot be empty")
	}

	key, expiresAt := jwtBlacklistDateBucket(claims.ExpiresAt.Time, "jwt")
	return key, expiresAt, nil
}

func jwtBlacklistDateBucket(expiresAt time.Time, args ...any) (string, time.Time) {
	expiresAt = expiresAt.UTC()
	bucket := time.Date(expiresAt.Year(), expiresAt.Month(), expiresAt.Day(), 0, 0, 0, 0, time.UTC)
	keys := append([]any(nil), args...)
	keys = append(keys, bucket.Format(jwtBlacklistBucketLayout))

	return KeyBlacklist(keys...), bucket.AddDate(0, 0, 1)
}

// MakeJWToken signs the supplied claims. Refresh tokens are registered when whitelist mode is enabled.
func MakeJWToken(claims contractauth.Claims, secrets ...string) (string, error) {
	token, signedClaims, err := makeJWToken(claims, secrets...)
	if err != nil {
		return "", err
	}

	if signedClaims.Type == JWTTypeRefresh && RefreshMode() == RefreshModeWhitelist {
		if err := registerRefreshToken(context.Background(), &signedClaims); err != nil {
			return "", err
		}
	}

	return token, nil
}

func makeJWToken(claims contractauth.Claims, secrets ...string) (string, contractauth.Claims, error) {
	secret, err := Secret(secrets...)
	if err != nil {
		return "", contractauth.Claims{}, err
	}

	if claims.Issuer == "" {
		return "", contractauth.Claims{}, errors.New("issuer cannot be empty")
	}
	if claims.Subject == "" {
		return "", contractauth.Claims{}, errors.New("subject cannot be empty")
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Unix() <= 0 {
		return "", contractauth.Claims{}, errors.New("IssuedAt cannot be empty")
	}
	if claims.NotBefore == nil || claims.NotBefore.Unix() <= 0 {
		return "", contractauth.Claims{}, errors.New("NotBefore cannot be empty")
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Unix() <= 0 {
		return "", contractauth.Claims{}, errors.New("ExpiresAt cannot be empty")
	}
	if !claims.ExpiresAt.Time.After(claims.IssuedAt.Time) {
		return "", contractauth.Claims{}, errors.New("invalid token lifetime")
	}

	claims.ID = id(claims.IssuedAt.Time, claims.Issuer, claims.Subject)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", contractauth.Claims{}, err
	}

	return token, claims, nil
}

// CheckAccessToken strictly validates an access token.
func CheckAccessToken(claims *contractauth.Claims, token string, secrets ...string) error {
	if err := parseJWToken(claims, token, secrets...); err != nil {
		return err
	}

	if claims.Type != "" && claims.Type != JWTTypeAccess {
		return fmt.Errorf("%w: expected %s", ErrInvalidJWTType, JWTTypeAccess)
	}

	return nil
}

// ValidateAccessToken validates an access token and checks its dated Bloom blacklist bucket.
func ValidateAccessToken(ctx context.Context, claims *contractauth.Claims, token string, secrets ...string) error {
	if err := CheckAccessToken(claims, token, secrets...); err != nil {
		return err
	}

	blacklisted, err := checkJWTBlacklist(ctx, claims)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrJWTBlacklistUnavailable, err)
	}
	if blacklisted {
		return ErrJWTBlacklisted
	}

	return nil
}

// CheckJWToken is the access-token compatibility entry point. Refresh is always false.
func CheckJWToken(claims *contractauth.Claims, token string, secrets ...string) (bool, error) {
	return false, CheckAccessToken(claims, token, secrets...)
}

func parseJWToken(claims *contractauth.Claims, token string, secrets ...string) error {
	if claims == nil {
		return errors.New("claims cannot be null")
	}
	if token == "" {
		return errors.New("token cannot be empty")
	}

	secret, err := Secret(secrets...)
	if err != nil {
		return err
	}

	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(jwtIssuer()),
		jwt.WithIssuedAt(),
		jwt.WithNotBeforeRequired(),
	)
	if err != nil {
		return err
	}
	if !parsed.Valid {
		return errors.New("token is invalid")
	}
	if claims.Subject == "" || claims.ID == "" || claims.IssuedAt == nil || claims.NotBefore == nil {
		return errors.New("token claims are incomplete")
	}

	return nil
}

// RefreshJWToken is retained only so existing callers fail explicitly instead of refreshing unsigned Claims.
// Deprecated: use RefreshJWTokens.
func RefreshJWToken(context.Context, *contractauth.Claims, ...int64) (string, error) {
	return "", ErrRefreshJWTokenUnsupported
}

// Secret returns the configured JWT signing secret or the first non-empty override.
func Secret(secrets ...string) (string, error) {
	secret := facades.Config().GetString("jwt.secret")
	secrets = lo.Filter(secrets, func(item string, _ int) bool {
		return lo.IsNotEmpty(item)
	})
	if len(secrets) > 0 {
		secret = secrets[0]
	}
	if secret == "" {
		return "", errors.New("secret cannot be empty")
	}

	return secret, nil
}

// Issuer prefixes an issuer with the configured application name.
func Issuer(issuer string) string {
	prefix := facades.Config().GetString("app.name") + ":"
	if strings.HasPrefix(issuer, prefix) {
		return issuer
	}

	return prefix + issuer
}

func jwtIssuer() string {
	return Issuer(facades.Config().GetString("jwt.sub", "default"))
}

func id(now time.Time, issuer, subject string) string {
	value := fmt.Sprintf("%s:%s:%d:%s", issuer, subject, now.Unix(), lo.RandomString(32, lo.AlphanumericCharset))
	return dongle.Hash.FromString(value).ByMd5().ToHexString()
}
