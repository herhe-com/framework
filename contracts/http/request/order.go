package request

type Order struct {
	Order uint8 `form:"order" json:"order" default:"50" validate:"gte=1,lte=99" label:"序号"`
}
