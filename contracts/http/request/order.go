package request

// Order is the sort weight input shared by sortable resources.
type Order struct {
	// Order is the sort weight: a larger value is listed first. Optional,
	// defaults to 50 when omitted, and must be between 1 and 99 when provided.
	Order uint8 `form:"order" json:"order" default:"50" validate:"gte=1,lte=99" label:"序号" example:"50"`
}
