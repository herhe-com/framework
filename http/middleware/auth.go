package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/http"
)

func Auth() app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		if !auth.Check(ctx) {
			ctx.Abort()
			http.Unauthorized(ctx)
			return
		}

		if auth.CheckBlacklist(c, auth.BlacklistOfJwtName(ctx)) {
			ctx.Abort()
			http.Unauthorized(ctx)
			return
		}

		ctx.Next(c)
	}
}
