package request

type IDOfUint struct {
	ID uint `json:"id" path:"id" form:"id" query:"id" validate:"required,gt=0" label:"ID"`
}

type IDOfUintEmpty struct {
	ID uint `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,gt=0" label:"ID"`
}

type IDOfUint64 struct {
	ID uint64 `json:"id" path:"id" form:"id" query:"id" validate:"required,gt=0" label:"ID"`
}

type IDOfUint64Empty struct {
	ID uint64 `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,gte=0" label:"ID"`
}

type IDOfSnowflake struct {
	ID string `json:"id" path:"id" form:"id" query:"id" validate:"required,snowflake" label:"ID"`
}

type IDOfSnowflakeEmpty struct {
	ID string `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,snowflake" label:"ID"`
}
