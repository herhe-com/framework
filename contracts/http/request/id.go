package request

type IDOfUint struct {
	ID uint `json:"id" path:"id" form:"id" query:"id" valid:"required,gt=0" label:"ID"`
}

type IDOfUintEmpty struct {
	ID uint `json:"id" path:"id" form:"id" query:"id" valid:"required,gt=0" label:"ID"`
}

type IDOfSnowflake struct {
	ID string `json:"id" path:"id" form:"id" query:"id" valid:"required,snowflake" label:"ID"`
}

type IDOfSnowflakeEmpty struct {
	ID string `json:"id" path:"id" form:"id" query:"id" valid:"omitempty,snowflake" label:"ID"`
}
