package auth

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"time"
)

func Role(ctx context.Context, c *app.RequestContext, user string) (role *auth.RoleOfCache, err error) {

	key := "ROLE:CACHE"

	result, ex := c.Get(key)

	if !ex {

		var res auth.RoleOfCache

		if err = facades.Redis.Get(ctx, RoleOfName(user)).Scan(&res); err != nil {
			return nil, err
		}

		role = &res

		c.Set(key, role)
	} else {
		role = result.(*auth.RoleOfCache)
	}

	return role, nil
}

func DoRole(ctx context.Context, platform uint16, id, clique *string, user, name string, temp bool, backs ...auth.RoleOfCache) (err error) {

	data := auth.RoleOfCache{
		User:     user,
		Id:       id,
		Name:     name,
		Clique:   clique,
		Platform: platform,
		Temp:     temp,
	}

	if len(backs) > 0 {
		data.Bak = &backs[0]
	}

	lifetime := facades.Cfg.GetInt("jwt.lifetime")

	expired := time.Hour * 2 * time.Duration(lifetime)

	if _, err = facades.Redis.Set(ctx, RoleOfName(user), &data, expired).Result(); err != nil {
		return err
	}

	if _, err = facades.Redis.Expire(ctx, RoleOfName(user), expired).Result(); err != nil {
		return err
	}

	return nil
}

//func DoRoleOfSet(ctx context.Context, data auth.RoleOfCache) (err error) {
//
//	str, _ := json.Marshal(data)
//
//	lifetime := facades.Cfg.GetInt("jwt.lifetime")
//
//	expired := time.Hour * 2 * time.Duration(lifetime)
//
//	_, err = facades.Redis.Set(ctx, RoleOfName(data.User), string(str)).Result()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func DoRoleOfRefresh(ctx context.Context, user string) (err error) {

	lifetime := facades.Cfg.GetInt("jwt.lifetime")

	expired := time.Hour * 2 * time.Duration(lifetime)

	_, err = facades.Redis.Expire(ctx, RoleOfName(user), expired).Result()
	if err != nil {
		return err
	}

	return nil
}

func RoleOfName(id string) string {

	name := facades.Cfg.GetString("server.name")

	return name + ":" + "role" + ":" + id
}
