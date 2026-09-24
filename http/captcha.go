package http

import (
	"github.com/cloudwego/hertz/pkg/app"

	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
	contracthttp "github.com/herhe-com/framework/contracts/http"
)

// CaptchaData returns the captcha data verified by captcha middleware.
func CaptchaData(ctx *app.RequestContext) (contractcaptcha.VerifyData, bool) {
	value, exists := ctx.Get(contracthttp.ContextOfCaptcha)
	if !exists {
		return contractcaptcha.VerifyData{}, false
	}
	data, ok := value.(contractcaptcha.VerifyData)
	return data, ok
}
