package auth

import (
	"errors"
	"strings"

	"github.com/herhe-com/framework/contracts/auth"
	"github.com/herhe-com/framework/facades"
	"github.com/samber/lo"
)

func permissionFunc() (auth.PermissionFunc, error) {

	permissions, ok := facades.Config().Get("auth.permissions").(func() []auth.Permission)

	if !ok || permissions == nil {
		return nil, errors.New("the value of auth.permissions must be a permission function")
	}

	return permissions, nil
}

func toTrees() error {

	if _, err := permissionFunc(); err != nil {
		return err
	}

	platforms, _ := facades.Config().Get("auth.platforms", []uint16{CodeOfPlatform}).([]uint16)

	mark := lo.CountBy(platforms, func(item uint16) bool {
		return !lo.Contains([]uint16{CodeOfPlatform, CodeOfClique, CodeOfRegion, CodeOfStore}, item)
	})

	if mark > 0 {
		return errors.New("platform configuration failed")
	}

	return nil
}

func Trees(platform uint16, ep bool, permissions ...[]string) []auth.Tree {

	permissionFunc, err := permissionFunc()
	if err != nil {
		return nil
	}

	var permission []string

	if len(permissions) > 0 {
		permission = permissions[0]
	}

	var trees []auth.Tree

	platforms, _ := facades.Config().Get("auth.platforms", []uint16{CodeOfPlatform}).([]uint16)

	trees = doTrees(permissionFunc(), nil, platforms)

	return filter(trees, platform, permission, ep)
}

func Modules(platform uint16) []auth.Module {

	permissionFunc, err := permissionFunc()

	if err != nil {
		return nil
	}

	platforms, _ := facades.Config().Get("auth.platforms", []uint16{CodeOfPlatform}).([]uint16)

	trees := doTrees(permissionFunc(), nil, platforms)

	return doModules(trees, platform)
}

// filter
//
//	@Description: 		过滤出权限树
//	@param trees		需要过滤的权限组
//	@param permissions	已存在的权限
//	@param ep		是否允许空已存权限
//	@return results
func filter(trees []auth.Tree, platform uint16, permissions []string, ep bool) (results []auth.Tree) {

	results = make([]auth.Tree, 0, len(trees))

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

	trees = make([]auth.Tree, 0, len(permissions))

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

	modules = make([]auth.Module, 0, len(trees))

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

	list = make([]string, 0, len(permissions))

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
