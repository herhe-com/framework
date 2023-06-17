package request

type IDOfUint struct {
	ID uint `json:"id" path:"id" form:"id" query:"id" validate:"required,gt=0" label:"ID"`
}

type IDOfSnowflake struct {
	ID string `json:"id" path:"id" form:"id" query:"id" validate:"required,snowflake" label:"ID"`
}
