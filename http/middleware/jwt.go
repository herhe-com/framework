package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/auth"
	contractauth "github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
)

func Jwt() app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		token := ctx.GetHeader(auth.JwtOfAuthorization)

		if len(token) > 0 {

			var claims contractauth.Claims

			refresh, err := auth.CheckJWToken(&claims, string(token))

			if err == nil {
				ctx.Set(auth.ContextOfID, claims.Subject)
				ctx.Set(auth.ContextOfClaims, claims)
			}

			if platform := auth.DefaultPlatform(); platform > 0 {
				ctx.Set(auth.ContextOfPlatform, platform)
			}

			if refresh && claims.Refresh {

				var refreshToken string

				if refreshToken, err = auth.RefreshJWToken(c, &claims); err != nil {
					return
				}

				ctx.Set(auth.ContextOfID, claims.Subject)
				ctx.Set(auth.ContextOfClaims, claims)

				ctx.Header(auth.Authorization, refreshToken)

				//  获取令牌刷新后的操作
				if callback := facades.Config().Get("auth.callback.refresh"); callback != nil {

					if function, ok := callback.(func(c context.Context, ctx *app.RequestContext)); ok {
						function(c, ctx)
					}
				}
			}

			if callback := facades.Config().Get("auth.callback.jwt"); callback != nil {

				if function, ok := callback.(func(c context.Context, ctx *app.RequestContext)); ok {
					function(c, ctx)
				}
			}
		}

		ctx.Next(c)
	}
}
