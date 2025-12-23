package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
	"github.com/herhe-com/framework/support/util"
)

func Limiter(option *LimiterOption) app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		path := string(ctx.URI().Path())

		var limit int64 = 3

		expiration := time.Minute
		generator := util.Keys("limit", path, ctx.ClientIP())

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

			script := `
				local current = redis.call("INCR", KEYS[1])
				if current == 1 then
					redis.call("EXPIRE", KEYS[1], ARGV[2])
				end
				if current > tonumber(ARGV[1]) then
					return -1
				end
				return current
			`

			if res, err := facades.Redis.Default().Eval(c, script, []string{generator}, limit, expiration.Seconds()).Int(); err != nil {
				ctx.Abort()
				http.Fail(ctx, "Redis error!")
				return
			} else if res == -1 {
				ctx.Abort()
				http.Fail(ctx, "The operation is too frequent!")
				return
			}
		}
	}
}

type LimiterOption struct {
	Limit      int64
	Expiration time.Duration
	Generator  func(c context.Context, ctx *app.RequestContext) string
}
