package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
)

func Permission(permission string) app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		if ok, _ := facades.Casbin.HasRoleForUser(auth.NameOfUser(auth.ID(ctx)), auth.NameOfDeveloper()); ok {
			ctx.Next(c)
			return
		}

		permissions := []any{auth.NameOfUser(auth.ID(ctx)), permission}

		platform := auth.Platform(ctx)

		if platform > 0 {
			permissions = append(permissions, auth.SPlatform(ctx))
		}

		if ok, _ := facades.Casbin.Enforce(permissions...); !ok {
			ctx.Abort()
			http.Forbidden(ctx)
			return
		}

		ctx.Next(c)
	}
}
