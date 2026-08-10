package middleware

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"

	"github.com/herhe-com/framework/auth"
	contractauth "github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
)

func Jwt() app.HandlerFunc {

	return func(c context.Context, ctx *app.RequestContext) {

		accessToken := ctx.GetHeader(auth.JwtOfAuthorization)
		refreshToken := ctx.GetHeader(auth.JwtOfRefreshToken)
		var claims contractauth.Claims

		if len(accessToken) > 0 {
			err := auth.ValidateAccessToken(c, &claims, string(accessToken))
			if errors.Is(err, auth.ErrJWTBlacklistUnavailable) {
				hlog.CtxErrorf(c, "failed to check JWT blacklist: %v", err)
				ctx.Abort()
				http.ServerError(ctx, "authentication service unavailable")
				return
			}

			if err == nil {
				if len(refreshToken) > 0 {
					ctx.Abort()
					http.Unauthorized(ctx)
					return
				}

				if err := setJWTContext(c, ctx, claims); err != nil {
					ctx.Abort()
					http.Unauthorized(ctx)
					return
				}
				ctx.Next(c)
				return
			}
		}

		if len(refreshToken) > 0 {
			pair, err := auth.RefreshJWTokens(c, string(refreshToken))
			if err != nil {
				ctx.Abort()
				if errors.Is(err, auth.ErrRefreshServiceUnavailable) {
					hlog.CtxErrorf(c, "failed to refresh JWT: %v", err)
					http.ServerError(ctx, "authentication service unavailable")
				} else {
					http.Unauthorized(ctx)
				}
				return
			}

			if err := auth.CheckAccessToken(&claims, pair.AccessToken); err != nil {
				hlog.CtxErrorf(c, "failed to validate refreshed access token: %v", err)
				ctx.Abort()
				http.ServerError(ctx, "authentication service unavailable")
				return
			}

			value, err := json.Marshal(pair)
			if err != nil {
				hlog.CtxErrorf(c, "failed to encode refreshed JWT pair: %v", err)
				ctx.Abort()
				http.ServerError(ctx, "authentication service unavailable")
				return
			}

			if err := setJWTContext(c, ctx, claims); err != nil {
				ctx.Abort()
				http.Unauthorized(ctx)
				return
			}

			ctx.Header(auth.JwtOfTokenPair, string(value))
			ctx.Header("Cache-Control", "no-store")
			ctx.Header("Pragma", "no-cache")
		}

		ctx.Next(c)
	}
}

func setJWTContext(c context.Context, ctx *app.RequestContext, claims contractauth.Claims) error {
	ctx.Set(auth.ContextOfID, claims.Subject)
	ctx.Set(auth.ContextOfClaims, claims)

	if platform := auth.DefaultPlatform(); platform > 0 {
		ctx.Set(auth.ContextOfPlatform, platform)
	}

	switch function := facades.Config().Get("auth.callback.jwt").(type) {
	case func(context.Context, *app.RequestContext) error:
		return function(c, ctx)
	case func(context.Context, *app.RequestContext):
		function(c, ctx)
	}

	return nil
}
