package auth

import "encoding/json"

type RoleOfCache struct {
	User     string       `json:"user"`             // 当前用户 ID
	Id       *string      `json:"id,omitempty"`     // 当前「集团/商户/单店」ID
	Name     string       `json:"name"`             // 当前「平台/集团/商户/单店」名称
	Clique   *string      `json:"clique,omitempty"` // 所处集团ID
	Platform uint16       `json:"platform"`         // 平台类型
	Temp     bool         `json:"temp"`             // 临时用户角色
	Bak      *RoleOfCache `json:"bak,omitempty"`    // 返回上层
}

func (that *RoleOfCache) MarshalBinary() ([]byte, error) {
	return json.Marshal(that)
}

func (that *RoleOfCache) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, that)
}

func (that *RoleOfCache) Check() bool {
	return that.Platform > 0
}

func (that *RoleOfCache) CheckId() bool {
	return that.Id != nil
}

func (that *RoleOfCache) IsPlatform() bool {
	return that.Platform == CodeOfPlatform
}

func (that *RoleOfCache) IsClique() bool {
	return that.Platform == CodeOfClique
}

func (that *RoleOfCache) IsRegion() bool {
	return that.Platform == CodeOfRegion
}

func (that *RoleOfCache) IsStore() bool {
	return that.Platform == CodeOfStore
}

func (that *RoleOfCache) IsTemp() bool {
	return that.Temp
}

func (that *RoleOfCache) HasClique() bool {
	return that.Clique != nil
}

func (that *RoleOfCache) HasBak() bool {
	return that.Bak != nil
}
