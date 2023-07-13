package http

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/go-playground/validator/v10"
	"github.com/herhe-com/framework/contracts/http/response"
	"github.com/herhe-com/framework/validation"
	"net/http"
)

func String(ctx *app.RequestContext, format string, values ...any) {
	ctx.String(http.StatusOK, format, values...)
}

func Unauthorized(ctx *app.RequestContext) {
	ctx.JSON(http.StatusOK, response.Response[any]{
		Code:    40100,
		Message: "Unauthorized",
	})
}

func Forbidden(ctx *app.RequestContext) {
	ctx.JSON(http.StatusForbidden, response.Response[any]{
		Code:    40300,
		Message: "Forbidden",
	})
}

func NotFound(ctx *app.RequestContext, message string) {
	ctx.JSON(http.StatusOK, response.Response[any]{
		Code:    40400,
		Message: message,
	})
}

func BadRequest(ctx *app.RequestContext, message any, a ...any) {

	msg := "bad request"

	switch message.(type) {
	case error:

		if err, ok := message.(validator.ValidationErrors); ok {
			msg = validation.Error(err)
		} else {
			msg = fmt.Sprintf("%s: %v", msg, message)
		}
	case string:

		msg = message.(string)

		if len(a) > 0 {
			msg = fmt.Sprintf(msg, a...)
		}
	default:
		msg = fmt.Sprintf("%s: %v", msg, message)
	}

	ctx.JSON(http.StatusOK, response.Response[any]{
		Code:    40000,
		Message: msg,
	})
}

func Login(ctx *app.RequestContext) {
	ctx.JSON(http.StatusOK, response.Response[any]{
		Code:    40100,
		Message: "登陆失败",
	})
}

func Success[T any](ctx *app.RequestContext, data ...T) {

	responses := response.Response[T]{
		Code:    20000,
		Message: "Success",
	}

	if len(data) > 0 {
		responses.Data = data[0]
	}

	ctx.JSON(http.StatusOK, responses)
}

func Fail(ctx *app.RequestContext, message string, a ...any) {

	msg := message

	if len(a) > 0 {
		msg = fmt.Sprintf(message, a...)
	}

	ctx.JSON(http.StatusOK, response.Response[any]{
		Code:    60000,
		Message: msg,
	})
}
