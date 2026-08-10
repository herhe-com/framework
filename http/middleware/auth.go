package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
)

func Auth() app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		if !auth.Check(ctx) {
			ctx.Abort()
			http.Unauthorized(ctx)
			return
		}

		if validate, ok := facades.Config().Get("auth.callback.auth").(func(context.Context, *app.RequestContext) error); ok {
			if err := validate(c, ctx); err != nil {
				ctx.Abort()
				http.Unauthorized(ctx)
				return
			}
		}

		ctx.Next(c)
	}
}
