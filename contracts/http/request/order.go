package request

type Order struct {
	Order uint8 `form:"order" json:"order" validate:"gte=1,lte=99" label:"序号"`
}
