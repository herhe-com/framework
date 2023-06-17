package auth

import (
	"github.com/herhe-com/framework/contracts/auth"
	"strings"
)

// HandlerPermissionsByTrees 将权限处理成可用权限树。
// permissions：需要处理的权限；
// platform：所处平台；
// codes：需要添加的权限 CODE；
// pms：当前用户权限；
// all：忽略用户权限校验，返回所有可用权限；
// caches：被转化成 map 类型的用户权限，默认不传。程序自动从 pms 转化；
func HandlerPermissionsByTrees(permissions []auth.Permission, platform uint16, codes, pms []string, all bool, caches ...map[string]string) (results []auth.PermissionOfTrees) {

	var cache map[string]string

	if !all && len(caches) <= 0 {
		cache = make(map[string]string, 0)

		for _, item := range pms {
			cache[item] = item
		}
	} else if len(caches) > 0 {
		cache = caches[0]
	}

	for _, item := range permissions {

		code := append(codes, item.Code)

		result := auth.PermissionOfTrees{
			Code:     strings.Join(code, "."),
			Name:     item.Name,
			Children: HandlerPermissionsByTrees(item.Children, platform, code, nil, all, cache),
		}

		// 标记该权限是否有该平台
		markPlatform := false
		markPermission := false

		if item.Common {
			markPlatform = true
		} else if len(item.Platforms) > 0 {
			//	判断底层权限是否包含该平台
			for _, value := range item.Platforms {
				if value == platform {
					markPlatform = true
				}
			}
		} else if len(result.Children) > 0 {
			//	判断中层权限目录是否有子内容返回
			markPlatform = true
			markPermission = true
		}

		if markPlatform && !markPermission {

			if all {
				markPermission = true
			} else {
				if _, ok := cache[result.Code]; ok {
					markPermission = true
				}
			}
		}

		if markPlatform && markPermission {
			results = append(results, result)
		}
	}

	return results
}

// HandlerPermissions 将权限处理成权限列表。
// permissions：需要处理的权限；
// platform：所处平台；
// codes：需要添加的权限 CODE；
// pms：当前用户权限；
// all：忽略用户权限校验，返回所有可用权限；
// caches：被转化成 map 类型的用户权限，默认不传。程序自动从 pms 转化；
func HandlerPermissions(permissions []auth.Permission, platform uint16, codes, pms []string, all bool) (results []auth.PermissionsOfSimple) {

	handlers := HandlerPermissionsByTrees(permissions, platform, codes, pms, all)

	return HandlerPermissionsOfEnd(handlers)
}

func HandlerPermissionsByParent(permissions []auth.Permission, platform uint16, codes []string) (results []auth.PermissionsOfSimple) {

	handlers := HandlerPermissionsByTrees(permissions, platform, nil, nil, true)

	return handlerPermissionByParent(handlers, codes, false)
}

func handlerPermissionByParent(permissions []auth.PermissionOfTrees, codes []string, parent bool, caches ...map[string]string) (results []auth.PermissionsOfSimple) {

	var cache map[string]string
	//	将 Code 数组转化为 Map 提高查询速度
	if len(caches) > 0 {
		cache = caches[0]
	} else {
		cache = make(map[string]string)
		for _, item := range codes {
			cache[item] = item
		}
	}

	for _, item := range permissions {

		p := parent

		//	查询权限是否被包含
		if _, ok := cache[item.Code]; ok {
			if len(item.Children) > 0 {
				//	子级下的所有权限均为授权权限
				p = true
			} else {
				//	本身已为最终授权权限，直接赋值通过
				results = append(results, auth.PermissionsOfSimple{
					Code: item.Code,
					Name: item.Name,
				})
			}
		}

		if len(item.Children) > 0 {
			results = append(results, handlerPermissionByParent(item.Children, codes, p, cache)...)
		} else if parent {
			results = append(results, auth.PermissionsOfSimple{
				Code: item.Code,
				Name: item.Name,
			})
		}

	}

	return results
}

func HandlerPermissionsOfEnd(permissions []auth.PermissionOfTrees) (results []auth.PermissionsOfSimple) {

	for _, item := range permissions {
		if len(item.Children) <= 0 {
			results = append(results, auth.PermissionsOfSimple{
				Code: item.Code,
				Name: item.Name,
			})
		} else {
			results = append(results, HandlerPermissionsOfEnd(item.Children)...)
		}
	}

	return results
}
