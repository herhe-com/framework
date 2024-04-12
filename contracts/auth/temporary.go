package auth

import "encoding/json"

type RoleOfTemporary struct {
	Org          string           `json:"org"` // 借位组织 ID
	Organization string           `json:"organization"`
	Platform     uint16           `json:"platform"` // 平台类型
	Clique       *string          `json:"clique,omitempty"`
	Bak          *RoleOfTemporary `json:"bak,omitempty"` // 返回上层
}

func (that *RoleOfTemporary) MarshalBinary() ([]byte, error) {
	return json.Marshal(that)
}

func (that *RoleOfTemporary) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, that)
}

func (that *RoleOfTemporary) Check() bool {
	return that.Platform > 0
}

func (that *RoleOfTemporary) IsPlatform() bool {
	return that.Platform == CodeOfPlatform
}

func (that *RoleOfTemporary) IsClique() bool {
	return that.Platform == CodeOfClique
}

func (that *RoleOfTemporary) IsRegion() bool {
	return that.Platform == CodeOfRegion
}

func (that *RoleOfTemporary) IsStore() bool {
	return that.Platform == CodeOfStore
}

func (that *RoleOfTemporary) HasBak() bool {
	return that.Bak != nil
}
