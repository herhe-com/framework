package request

// IDOfUint identifies a resource by a required unsigned integer ID, usually
// bound from a path or query parameter.
type IDOfUint struct {
	// ID is the resource primary key. Required, must be greater than 0.
	ID uint `json:"id" path:"id" form:"id" query:"id" validate:"required,gt=0" label:"ID" example:"1"`
}

// IDOfUintEmpty identifies a resource by an optional unsigned integer ID.
type IDOfUintEmpty struct {
	// ID is the resource primary key. Optional: when provided it must be greater
	// than 0.
	ID uint `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,gt=0" label:"ID" example:"1"`
}

// IDOfUint64 identifies a resource by a required unsigned 64-bit ID.
type IDOfUint64 struct {
	// ID is the resource primary key. Required, must be greater than 0.
	ID uint64 `json:"id" path:"id" form:"id" query:"id" validate:"required,gt=0" label:"ID" example:"1"`
}

// IDOfUint64Empty identifies a resource by an optional unsigned 64-bit ID.
type IDOfUint64Empty struct {
	// ID is the resource primary key. Optional: when provided it must be greater
	// than or equal to 0.
	ID uint64 `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,gte=0" label:"ID" example:"1"`
}

// IDOfSnowflake identifies a resource by a required snowflake ID, which is a
// 19-digit decimal string rather than a number.
type IDOfSnowflake struct {
	// ID is the resource primary key as a snowflake string. Required, must be a
	// valid snowflake ID.
	ID string `json:"id" path:"id" form:"id" query:"id" validate:"required,snowflake" label:"ID" example:"1876543210987654321"`
}

// IDOfSnowflakeEmpty identifies a resource by an optional snowflake ID.
type IDOfSnowflakeEmpty struct {
	// ID is the resource primary key as a snowflake string. Optional: when
	// provided it must be a valid snowflake ID.
	ID string `json:"id" path:"id" form:"id" query:"id" validate:"omitempty,snowflake" label:"ID" example:"1876543210987654321"`
}
