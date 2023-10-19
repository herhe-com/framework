package middleware

import (
	"context"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
	"time"
)

func Limiter(option *LimiterOption) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {

		path := string(ctx.URI().Path())

		var limit int64 = 3
		expiration := time.Minute
		generator := fmt.Sprintf("%s:limit:%s:%s", facades.Cfg.GetString("app.name"), path, ctx.ClientIP())

		if option != nil {

			if option.Limit > 0 {
				limit = option.Limit
			}

			if option.Expiration > 0 {
				expiration = option.Expiration
			}

			if option.Generator != nil {
				generator = option.Generator(c, ctx)
			}
		}

		if facades.Redis != nil {

			total, err := facades.Redis.Incr(c, generator).Result()

			if err != nil || total > limit {
				ctx.Abort()
				http.Fail(ctx, "The operation is too frequent!")
				return
			}

			if total == 1 {
				facades.Redis.Expire(c, generator, expiration)
			}
		}
	}
}

type LimiterOption struct {
	Limit      int64
	Expiration time.Duration
	Generator  func(c context.Context, ctx *app.RequestContext) string
}
