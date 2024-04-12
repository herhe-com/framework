package auth

import (
	"github.com/cloudwego/hertz/pkg/app"
	authConstant "github.com/herhe-com/framework/contracts/auth"
	"strconv"
)

func Check(ctx *app.RequestContext) bool {

	if ID(ctx) != "" {
		return true
	}

	return false
}

func ID(ctx *app.RequestContext) string {

	if value, ok := ctx.Get(authConstant.ContextOfID); ok {
		return value.(string)
	}

	return ""
}

func Platform(ctx *app.RequestContext) (platform uint16) {

	if value, exits := ctx.Get(authConstant.ContextOfPlatform); exits {
		platform, _ = value.(uint16)
	}

	return platform
}

func SPlatform(ctx *app.RequestContext) string {

	platform := Platform(ctx)

	if platform > 0 {
		return strconv.Itoa(int(platform))
	} else {
		return ""
	}
}

func Organization(ctx *app.RequestContext) (platform *string) {

	if value, exits := ctx.Get(authConstant.ContextOfOrganization); exits {
		platform, _ = value.(*string)
	}

	return platform
}

func Clique(ctx *app.RequestContext) (clique *string) {

	if value, exits := ctx.Get(authConstant.ContextOfClique); exits {
		clique, _ = value.(*string)
	}

	return clique
}

func Claims(ctx *app.RequestContext) (claims *authConstant.Claims) {

	if value, ok := ctx.Get(authConstant.ContextOfClaims); ok {
		if claim, o := value.(authConstant.Claims); o {
			return &claim
		}
	}

	return nil
}
