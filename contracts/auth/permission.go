package auth

// Permission
// 一层为模块层，底层为权限层，中间部分为目录层「忽律权限功能」
// Name 名称
// Common 是否通用接口，不再校验平台参数
// Platforms 适用平台
type Permission struct {
	Code      string
	Name      string
	Common    bool
	Platforms []uint16
	Children  []Permission
}

type PermissionOfTrees struct {
	Code     string              `json:"code"`
	Name     string              `json:"name"`
	Children []PermissionOfTrees `json:"children,omitempty"`
}

type PermissionsOfSimple struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
