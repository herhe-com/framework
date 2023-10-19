package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-module/carbon/v2"
	"github.com/golang-module/dongle"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/support/str"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"strings"
	"time"
)

func MakeJWT(claims jwt.RegisteredClaims, secrets ...string) (token string, err error) {

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

func CheckJWT(claims *jwt.RegisteredClaims, token, iss string, secrets ...string) (refresh bool, err error) {

	var secret string

	if secret, err = Secret(secrets...); err != nil {
		return false, err
	}

	_, err = jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})

	var valid *jwt.ValidationError

	ok := errors.As(err, &valid)

	if err != nil && !ok {
		return false, err
	}

	if err == nil || valid.Errors > 0 && valid.Is(jwt.ErrTokenExpired) {

		now := carbon.Now()

		if !claims.VerifyIssuer(Issuer(iss), true) {
			return false, jwt.ErrTokenUsedBeforeIssued
		}

		if !claims.VerifyNotBefore(now.ToStdTime(), true) {
			return false, jwt.ErrTokenUsedBeforeIssued
		}

		if !claims.VerifyExpiresAt(now.ToStdTime(), true) {

			lifetime := claims.IssuedAt.Sub(claims.ExpiresAt.Time).Seconds()

			if claims.VerifyExpiresAt(now.SubSeconds(int(lifetime)).ToStdTime(), true) {
				return true, nil
			}

			return false, jwt.ErrTokenExpired
		}
	}

	return false, nil
}

func RefreshJWT(ctx context.Context, claims *jwt.RegisteredClaims, leeways ...int64) (token string, err error) {

	if lo.IsEmpty(claims) {
		return "", errors.New("claims cannot be empty")
	}

	bk := blacklist(claims.Issuer, claims.Subject)

	var blacklists map[string]string

	blacklists, err = facades.Redis.HGetAll(ctx, bk).Result()

	now := carbon.Now()

	if errors.Is(err, redis.Nil) || len(blacklists) <= 0 {

		lifetime := claims.IssuedAt.Sub(claims.ExpiresAt.Time).Seconds()

		claims.IssuedAt = jwt.NewNumericDate(now.ToStdTime())
		claims.NotBefore = jwt.NewNumericDate(now.ToStdTime())
		claims.ExpiresAt = jwt.NewNumericDate(now.AddSeconds(int(lifetime)).ToStdTime())

		if token, err = MakeJWT(*claims); err != nil {
			return "", err
		}

		if facades.Redis != nil {

			var affected int64

			if affected, err = facades.Redis.HSet(ctx, bk, "token", token, "created_at", now.ToDateTimeString()).Result(); err != nil || affected <= 0 {
				return "", err
			}

			facades.Redis.ExpireAt(ctx, bk, carbon.CreateFromStdTime(claims.ExpiresAt.Time).AddSeconds(int(lifetime)).ToStdTime())
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

	prefix := facades.Cfg.GetString("app.name") + ":"

	if strings.HasPrefix(issuer, prefix) {
		return issuer
	}

	return prefix + issuer
}

func id(now time.Time, issuer, subject string) string {

	s := fmt.Sprintf("%s:%s:%d:%s", issuer, subject, now.Unix(), str.Random(32))

	return dongle.Encrypt.FromString(s).ByMd5().ToHexString()
}

func blacklist(iss, sub string) string {
	return iss + ":" + sub
}
