package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/herhe-com/framework/facades"
	"time"
)

func AccessMiddleware() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {

		if facades.Cfg.GetBool("app.debug") {
			start := time.Now()
			ctx.Next(c)
			end := time.Now()
			latency := end.Sub(start).Microseconds
			hlog.CtxTracef(c, "status=%d cost=%d method=%s full_path=%s client_ip=%s",
				ctx.Response.StatusCode(), latency,
				ctx.Request.Header.Method(), ctx.Request.URI().PathOriginal(), ctx.ClientIP())
		}
	}
}
