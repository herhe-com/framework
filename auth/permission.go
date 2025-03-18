package auth

import (
	"errors"
	"fmt"
	"github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/samber/lo"
	"strings"
)

func toTrees() error {

	var trees []auth.Tree

	permissions, ok := facades.Cfg.Get("auth.permissions").([]auth.Permission)

	if ok {

		var prefix []string

		platforms, _ := facades.Cfg.Get("auth.platforms", []uint16{auth.CodeOfStore}).([]uint16)

		mark := lo.CountBy(platforms, func(item uint16) bool {
			return !lo.Contains([]uint16{auth.CodeOfPlatform, auth.CodeOfClique, auth.CodeOfStore}, item)
		})

		if mark > 0 {
			return errors.New("platform configuration failed")
		}

		platforms = append(platforms, auth.CodeOfRegion)

		trees = doTrees(permissions, prefix, platforms)

		if len(trees) > 0 {

			for _, platform := range platforms {

				key := fmt.Sprintf("%s.module.%d", "auth", platform)

				facades.Cfg.Set(key, doModules(trees, platform))
			}
		}

		facades.Cfg.Set("auth.trees", trees)
	}

	return nil
}

func Trees(platform uint16, ep bool, permissions ...[]string) []auth.Tree {

	all, _ := facades.Cfg.Get("auth.trees").([]auth.Tree)

	var permission []string

	if len(permissions) > 0 {
		permission = permissions[0]
	}

	return filter(all, platform, permission, ep)
}

func Modules(platform uint16) []auth.Module {

	key := fmt.Sprintf("%s.module.%d", "auth", platform)

	modules, _ := facades.Cfg.Get(key).([]auth.Module)

	return modules
}

// filter
//
//	@Description: 		过滤出权限树
//	@param trees		需要过滤的权限组
//	@param permissions	已存在的权限
//	@param ep		是否允许空已存权限
//	@return results
func filter(trees []auth.Tree, platform uint16, permissions []string, ep bool) (results []auth.Tree) {

	for _, item := range trees {

		mark := false

		if len(item.Children) > 0 {

			item.Children = filter(item.Children, platform, permissions, ep)

			if len(item.Children) > 0 {
				mark = true
			}
		} else if platform > 0 && lo.Contains(item.Platforms, platform) {

			if ep {
				mark = true
			} else if lo.Contains(permissions, item.Code) {
				mark = true
			}

		} else if !ep && lo.Contains(permissions, item.Code) {
			mark = true
		}

		if mark {
			item.Platforms = nil
			results = append(results, item)
		}
	}

	return results
}

func doTrees(permissions []auth.Permission, prefix []string, defaultPlatforms []uint16) (trees []auth.Tree) {

	for _, item := range permissions {

		codes := append(prefix, item.Code)

		tree := auth.Tree{
			Name:     item.Name,
			Code:     strings.Join(codes, "."),
			Children: doTrees(item.Children, codes, defaultPlatforms),
		}

		if len(tree.Children) <= 0 {

			if item.Common {
				tree.Platforms = defaultPlatforms
			} else if len(item.Platforms) > 0 {
				tree.Platforms = item.Platforms
			} else {
				tree.Platforms = defaultPlatforms
			}
		}

		trees = append(trees, tree)
	}

	return trees
}

func doModules(trees []auth.Tree, platform uint16) (modules []auth.Module) {

	for _, item := range trees {

		permissions := doList(item.Children, platform)

		if len(permissions) > 0 {

			module := auth.Module{
				Code:        item.Code,
				Name:        item.Name,
				Permissions: permissions,
			}

			modules = append(modules, module)
		}
	}

	return modules
}

func doList(permissions []auth.Tree, platform uint16) (list []string) {

	for _, item := range permissions {

		if len(item.Children) > 0 {

			if resp := doList(item.Children, platform); len(resp) > 0 {
				list = append(list, resp...)
			}
		} else if lo.Contains(item.Platforms, platform) {
			list = append(list, item.Code)
		}
	}

	return list
}
