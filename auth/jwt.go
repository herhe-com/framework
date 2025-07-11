package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/dromara/carbon/v2"
	"github.com/dromara/dongle"
	"github.com/golang-jwt/jwt/v4"
	"github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"strings"
	"time"
)

// NewJWToken
//
//	@Description: 生成 JWT
//	@param id 用户
//	@param lifetime 生存时间（分钟）
//	@param ref 	是否可被刷新
//	@param ext	扩展变量
//
// NewJWToken
func NewJWToken(id string, lifetime int, refresh bool, ext map[string]any) (token string, err error) {

	sub := facades.Cfg.GetString("jwt.sub")

	now := carbon.Now()

	claims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer(sub),
			Subject:   id,
			IssuedAt:  jwt.NewNumericDate(now.StdTime()),
			NotBefore: jwt.NewNumericDate(now.StdTime()),
			ExpiresAt: jwt.NewNumericDate(now.AddMinutes(lifetime).StdTime()),
		},
		Refresh: refresh,
		Ext:     ext,
	}

	return MakeJWToken(claims)
}

func BlacklistOfJwtName(ctx *app.RequestContext) string {
	return KeyBlacklist("jwt", Claims(ctx).ID)
}

func BlacklistOfJwtValue(c context.Context, ctx *app.RequestContext) (bool, error) {

	claims := Claims(ctx)

	if claims == nil {
		return true, errors.New("claims cannot be null")
	}

	if facades.Redis == nil {
		return true, errors.New("redis cannot be null")
	}

	now := carbon.Now()

	expires := Claims(ctx).ExpiresAt.Sub(now.StdTime()) * time.Second

	maxExpired := time.Hour * 24 * 7

	if expires > maxExpired {
		expires = maxExpired
	}

	return Blacklist(c, now.Timestamp(), expires, BlacklistOfJwtName(ctx)), nil
}

func MakeJWToken(claims auth.Claims, secrets ...string) (token string, err error) {

	var secret string

	if secret, err = Secret(secrets...); err != nil {
		return "", err
	}

	if lo.IsEmpty(claims.Issuer) {
		return "", errors.New("issuer cannot be empty")
	}

	if lo.IsEmpty(claims.Subject) {
		return "", errors.New("subject cannot be empty")
	}

	if lo.IsEmpty(claims.IssuedAt) || claims.IssuedAt.Unix() <= 0 {
		return "", errors.New("IssuedAt cannot be empty")
	}

	if lo.IsEmpty(claims.NotBefore) || claims.NotBefore.Unix() <= 0 {
		return "", errors.New("NotBefore cannot be empty")
	}

	if lo.IsEmpty(claims.ExpiresAt) || claims.ExpiresAt.Unix() <= 0 {
		return "", errors.New("ExpiresAt cannot be empty")
	}

	claims.ID = id(claims.IssuedAt.Time, claims.Issuer, claims.Subject)

	if token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret)); err != nil {
		return
	}

	return token, nil
}

func CheckJWToken(claims *auth.Claims, token string, secrets ...string) (refresh bool, err error) {

	var secret string

	if secret, err = Secret(secrets...); err != nil {
		return false, err
	}

	_, err = jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})

	var valid *jwt.ValidationError

	if err != nil && (!errors.As(err, &valid) || !errors.Is(err, jwt.ErrTokenExpired)) {
		return false, err
	}

	now := carbon.Now()

	sub := facades.Cfg.GetString("jwt.sub")

	if !claims.VerifyIssuer(Issuer(sub), true) {
		return false, jwt.ErrTokenUsedBeforeIssued
	}

	if !claims.VerifyNotBefore(now.StdTime(), true) {
		return false, jwt.ErrTokenUsedBeforeIssued
	}

	if !claims.VerifyExpiresAt(now.StdTime(), true) {

		lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time).Seconds()

		if lifetime >= 86400*30 {
			lifetime = 86400 * 30
		}

		claims.ExpiresAt = jwt.NewNumericDate(claims.ExpiresAt.Add(time.Second * time.Duration(lifetime)))

		if claims.VerifyExpiresAt(now.StdTime(), true) {
			return true, jwt.ErrTokenExpired
		}

		return false, jwt.ErrTokenExpired
	}

	return false, nil
}

func RefreshJWToken(ctx context.Context, claims *auth.Claims, leeways ...int64) (token string, err error) {

	if lo.IsEmpty(claims) {
		return "", errors.New("claims cannot be empty")
	}

	bk := blacklist(claims.Issuer, claims.Subject)

	var blacklists map[string]string

	if facades.Redis != nil {
		blacklists, err = facades.Redis.HGetAll(ctx, bk).Result()
	}

	now := carbon.Now()

	if facades.Redis == nil || errors.Is(err, redis.Nil) || len(blacklists) <= 0 {

		lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time).Seconds()

		claims.IssuedAt = jwt.NewNumericDate(now.StdTime())
		claims.NotBefore = jwt.NewNumericDate(now.StdTime())
		claims.ExpiresAt = jwt.NewNumericDate(now.AddSeconds(int(lifetime)).StdTime())

		if token, err = MakeJWToken(*claims); err != nil {
			return "", err
		}

		if facades.Redis != nil {

			script := `
				local token = redis.call("HSET", KEYS[1], "token", ARGV[1])
				local created_at = redis.call("HSET", KEYS[1], "created_at", ARGV[2])
				if token and created_at then
					redis.call("EXPIREAT", KEYS[1], ARGV[3])
					return 1
				end
				return 0
				`

			expire := carbon.CreateFromStdTime(claims.ExpiresAt.Time).AddSeconds(int(lifetime))

			if result, err := facades.Redis.Eval(ctx, script, []string{bk}, token, now.ToDateTimeString(), expire.Timestamp()).Result(); err != nil {
				return "", err
			} else if fmt.Sprintf("%v", result) != "1" {
				return "", errors.New("failed to set the refresh token")
			}
		}

		return token, nil
	} else {

		var ok bool
		var created string

		if token, ok = blacklists["token"]; !ok {
			return "", errors.New("failed to read the refresh token")
		}

		if created, ok = blacklists["created_at"]; !ok {
			return "", errors.New("failed to read the refresh time")
		}

		diff := now.DiffAbsInSeconds(carbon.Parse(created))

		leeway := facades.Cfg.GetInt64("jwt.leeway")

		leeways = lo.Filter(leeways, func(item int64, index int) bool {
			return item > 0
		})

		if len(leeways) > 0 {
			leeway = leeways[0]
		}

		if diff > leeway {
			return "", errors.New("the token cannot be refreshed")
		}
	}

	return token, nil
}

func Secret(secrets ...string) (secret string, err error) {

	secret = facades.Cfg.GetString("jwt.secret")

	secrets = lo.Filter(secrets, func(item string, index int) bool {
		return lo.IsNotEmpty(item)
	})

	if len(secrets) > 0 {
		secret = secrets[0]
	}

	if lo.IsEmpty(secret) {
		return "", errors.New("secret cannot be empty")
	}

	return secret, nil
}

func Issuer(issuer string) string {

	prefix := facades.Cfg.GetString("server.name") + ":"

	if strings.HasPrefix(issuer, prefix) {
		return issuer
	}

	return prefix + issuer
}

func id(now time.Time, issuer, subject string) string {

	s := fmt.Sprintf("%s:%s:%d:%s", issuer, subject, now.Unix(), lo.RandomString(32, lo.AlphanumericCharset))

	return dongle.Encrypt.FromString(s).ByMd5().ToHexString()
}

func blacklist(iss, sub string) string {
	return iss + ":" + sub
}
