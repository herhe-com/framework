package request

// Enable is the enable/disable switch input shared by resources that support
// being turned on and off.
type Enable struct {
	// IsEnable is the target state: 1 enables the resource, 2 disables it.
	// Optional, defaults to 1 when omitted.
	IsEnable uint8 `json:"is_enable" form:"is_enable" default:"1" validate:"required,oneof=1 2" label:"是否启用" example:"1" enums:"1,2"`
}
