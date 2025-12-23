package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/herhe-com/framework/facades"
)

func Access() app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		if facades.Cfg.GetBool("app.debug") {

			start := time.Now()
			ctx.Next(c)
			end := time.Now()
			duration := end.Sub(start)
			hlog.CtxTracef(c, "status=%d cost=%s method=%s full_path=%s client_ip=%s",
				ctx.Response.StatusCode(), duration,
				ctx.Request.Header.Method(), ctx.Request.URI().PathOriginal(), ctx.ClientIP())
		}
	}
}
