package auth

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/redis/go-redis/v9"
)

func Temporary(c context.Context, ctx *app.RequestContext) (role *auth.RoleOfTemporary, err error) {

	key := "ROLE:TEMPORARY"

	result, ex := ctx.Get(key)

	if facades.Redis == nil {
		return nil, errors.New("please initialize Redis first")
	}

	if !ex {

		var res auth.RoleOfTemporary

		err = facades.Redis.Default().Get(c, RoleOfName(ID(ctx))).Scan(&res)

		if errors.Is(err, redis.Nil) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		role = &res

		ctx.Set(key, role)
	} else {
		role = result.(*auth.RoleOfTemporary)
	}

	return role, nil
}

func DoTemporary(c context.Context, ctx *app.RequestContext, platform uint16, org, organization string, clique *string, backs ...*auth.RoleOfTemporary) (err error) {

	data := auth.RoleOfTemporary{
		Platform:     platform,
		Org:          org,
		Organization: organization,
		Clique:       clique,
	}

	if len(backs) > 0 {
		data.Bak = backs[0]
	}

	lifetime := facades.Cfg.GetInt("jwt.lifetime")

	expired := time.Hour * 2 * time.Duration(lifetime)

	if _, err = facades.Redis.Default().Set(c, RoleOfName(ID(ctx)), &data, expired).Result(); err != nil {
		return err
	}

	return nil
}

func DoTemporaryOfDelete(c context.Context, ctx *app.RequestContext) (err error) {

	_, err = facades.Redis.Default().Del(c, RoleOfName(ID(ctx))).Result()

	if err != nil {
		return err
	}

	return nil
}

func DoTemporaryOfRefresh(c context.Context, ctx *app.RequestContext) (err error) {

	lifetime := facades.Cfg.GetInt("jwt.lifetime")

	expired := time.Hour * 2 * time.Duration(lifetime)

	_, err = facades.Redis.Default().Expire(c, RoleOfName(ID(ctx)), expired).Result()

	if err != nil {
		return err
	}

	return nil
}

func RoleOfName(id string) string {

	name := facades.Cfg.GetString("app.name")

	return name + ":" + "role" + ":" + id
}
