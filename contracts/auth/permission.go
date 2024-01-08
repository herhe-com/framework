package auth

type Module struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// Permission
// 底层为权限层，中间部分为目录层「忽律权限功能」
// Name 名称
// Common 是否通用接口，不再校验适用平台参数
// Platforms 适用平台
type Permission struct {
	Code      string
	Name      string
	Common    bool         `json:"common"`
	Platforms []uint16     `json:"platforms"`
	Children  []Permission `json:"children"`
}

type Tree struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Platforms []uint16 `json:"platforms,omitempty"`
	Children  []Tree   `json:"children,omitempty"`
}

type PermissionsOfSimple struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
