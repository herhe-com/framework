package request

type Enable struct {
	IsEnable uint8 `json:"is_enable" form:"is_enable" validate:"required,oneof=1 2" label:"是否启用"`
}
