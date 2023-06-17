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
	"strings"
)

func MakeJwtToken(iss, sub string, lifetime int, secret ...string) (signed string, err error) {

	sec := facades.Cfg.GetString("jwt.secret")

	if len(secret) > 0 {
		sec = secret[0]
	}

	if sec == "" {
		return "", errors.New("服务器密钥不能为空")
	}

	iss = Issuer(iss)

	now := carbon.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    iss,
		Subject:   sub,
		NotBefore: jwt.NewNumericDate(now.Carbon2Time()),
		IssuedAt:  jwt.NewNumericDate(now.Carbon2Time()),
		ExpiresAt: jwt.NewNumericDate(now.AddHours(lifetime).Carbon2Time()),
		ID:        id(now, iss, sub),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err = token.SignedString([]byte(sec))

	if err != nil {
		return
	}

	return signed, nil
}

func CheckJwtToken(ctx context.Context, authorization, iss string, lifetime int, secret ...string) (*jwt.RegisteredClaims, string, bool) {

	sec := facades.Cfg.GetString("jwt.secret")

	if len(secret) > 0 {
		sec = secret[0]
	}

	var claims jwt.RegisteredClaims

	_, err := jwt.ParseWithClaims(authorization, &claims, func(token *jwt.Token) (any, error) {
		return []byte(sec), nil
	})

	valid, ok := err.(*jwt.ValidationError)

	if err != nil && !ok {
		return nil, "", false
	}

	if claims.Subject == "" {
		return nil, "", false
	}

	if err == nil || valid.Errors > 0 && valid.Is(jwt.ErrTokenExpired) {

		now := carbon.Now()

		if !claims.VerifyIssuer(Issuer(iss), true) {
			return nil, "", false
		}

		if !claims.VerifyNotBefore(now.Carbon2Time(), true) {
			return nil, "", false
		}

		if claims.VerifyExpiresAt(now.Carbon2Time(), true) {
			return &claims, "", true
		}

		//	触发令牌刷新
		if claims.VerifyExpiresAt(now.SubHours(lifetime).Carbon2Time(), true) {

			bk := blacklist(claims.Issuer, claims.Subject)

			var blacklist map[string]string

			blacklist, err = facades.Redis.HGetAll(ctx, bk).Result()

			var refresh string

			if errors.Is(err, redis.Nil) || len(blacklist) <= 0 {

				refresh, err = MakeJwtToken(claims.Issuer, claims.Subject, lifetime)

				if err != nil {
					return nil, "", false
				}

				affected, err := facades.Redis.HSet(ctx, bk, "token", refresh, "created_at", now.ToDateTimeString()).Result()

				if err != nil || affected <= 0 {
					return nil, "", false
				}

				facades.Redis.ExpireAt(ctx, bk, carbon.Time2Carbon(claims.ExpiresAt.Time).AddHours(lifetime).Carbon2Time())

				return &claims, refresh, true
			} else {

				var created string

				if refresh, ok = blacklist["token"]; !ok {
					return nil, "", false
				}

				if created, ok = blacklist["created_at"]; !ok {
					return nil, "", false
				}

				diff := now.DiffAbsInSeconds(carbon.Parse(created))

				leeway := facades.Cfg.GetInt64("jwt.leeway")

				if diff <= leeway {
					return &claims, refresh, true
				}
			}

			return nil, "", false
		}
	}

	return nil, "", false
}

func Issuer(issuer string) string {

	prefix := facades.Cfg.GetString("app.name") + ":"

	if strings.HasPrefix(issuer, prefix) {
		return issuer
	}

	return prefix + issuer
}

func id(now carbon.Carbon, issuer, subject string) string {

	s := fmt.Sprintf("%s:%s:%d:%s", issuer, subject, now.Timestamp(), str.Random(32))

	return dongle.Encrypt.FromString(s).ByMd5().ToHexString()
}

func blacklist(iss, sub string) string {
	return iss + ":" + sub
}
