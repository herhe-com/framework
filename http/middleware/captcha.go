package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
	contracthttp "github.com/herhe-com/framework/contracts/http"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
)

// Captcha validates the captcha in the request for the supplied scene.
// It only verifies the answer; call Use after the protected business action succeeds.
func Captcha(scene string) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		data := contractcaptcha.VerifyData{Scene: scene}
		if err := ctx.Bind(&data); err != nil {
			ctx.Abort()
			http.BadRequest(ctx, err)
			return
		}
		data.Scene = scene

		application, ok := facades.Optional[contractcaptcha.Application]()
		if !ok || application == nil {
			ctx.Abort()
			http.ServerError(ctx, "captcha service unavailable")
			return
		}
		if err := application.Verify(c, data); err != nil {
			ctx.Abort()
			http.BadRequest(ctx, err)
			return
		}

		ctx.Set(contracthttp.ContextOfCaptcha, data)
		ctx.Next(c)
	}
}
