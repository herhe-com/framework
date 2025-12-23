package auth

import (
	"database/sql"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	authConstant "github.com/herhe-com/framework/contracts/auth"
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

func Organization(ctx *app.RequestContext) (id sql.NullString) {

	if value, exits := ctx.Get(authConstant.ContextOfOrganization); exits {
		if platform, ok := value.(string); ok {
			id = sql.NullString{
				String: platform,
				Valid:  true,
			}
		}
	}

	return id
}

func Clique(ctx *app.RequestContext) (id sql.NullString) {

	if value, exits := ctx.Get(authConstant.ContextOfClique); exits {
		if clique, ok := value.(string); ok && clique != "" {
			id = sql.NullString{
				String: clique,
				Valid:  true,
			}
		}
	}

	return id
}

func Claims(ctx *app.RequestContext) (claims *authConstant.Claims) {

	if value, exist := ctx.Get(authConstant.ContextOfClaims); exist {
		if tmp, ok := value.(authConstant.Claims); ok {
			claims = &tmp
		}
	}

	return claims
}
