package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/auth"
	authConstant "github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
)

func Jwt(sub string) app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		if token := ctx.GetHeader(authConstant.JwtOfAuthorization); len(token) > 0 {

			var claims authConstant.Claims

			refresh, err := auth.CheckJWToken(&claims, string(token), sub)

			if err == nil {
				ctx.Set(authConstant.ContextOfID, claims.Subject)
				ctx.Set(authConstant.ContextOfClaims, claims)
				ctx.Set(authConstant.ContextOfPlatform, claims.Platform)
				ctx.Set(authConstant.ContextOfOrganization, claims.Organization)
				ctx.Set(authConstant.ContextOfClique, claims.Clique)
			}

			if refresh && claims.Refresh {

				var refreshToken string

				if refreshToken, err = auth.RefreshJWToken(c, &claims); err != nil {
					return
				}

				ctx.Set(authConstant.ContextOfID, claims.Subject)
				ctx.Set(authConstant.ContextOfClaims, claims)
				ctx.Set(authConstant.ContextOfPlatform, claims.Platform)
				ctx.Set(authConstant.ContextOfOrganization, claims.Organization)
				ctx.Set(authConstant.ContextOfClique, claims.Clique)

				ctx.Header(authConstant.Authorization, refreshToken)

				//  获取令牌刷新后的操作
				if ref, ok := facades.Cfg.Get("auth.refresh").(func(co context.Context, rc *app.RequestContext)); ok {
					ref(c, ctx)
				}
			}

			if auth.Check(ctx) {

				if temporary, _ := auth.Temporary(c, ctx); temporary != nil {
					ctx.Set(authConstant.ContextOfPlatform, temporary.Platform)
					ctx.Set(authConstant.ContextOfOrganization, &temporary.Org)
					ctx.Set(authConstant.ContextOfClique, temporary.Clique)
				}
			}
		}
	}
}
