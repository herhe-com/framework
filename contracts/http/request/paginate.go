package request

// Paginate is the query input shared by list endpoints. Omitted values fall
// back to the GetPage/GetSize defaults.
type Paginate struct {
	// Page is the requested page number, starting from 1. Optional, defaults to
	// 1.
	Page int `form:"page" json:"page" query:"page" default:"1" validate:"omitempty,number,gte=0" label:"页码" example:"1"`
	// Size is the number of records per page. Optional, defaults to 15, must not
	// exceed 50.
	Size int `form:"size" json:"size" query:"size" default:"15" validate:"omitempty,number,gte=0,lte=50" label:"页数" example:"15"`
}

func (p *Paginate) GetPage() int {
	if p.Page > 0 {
		return p.Page
	}
	return 1
}

func (p *Paginate) GetSize() int {
	if p.Size > 0 {
		return p.Size
	}
	return 15
}

func (p *Paginate) GetLimit() int {
	return p.GetSize()
}

func (p *Paginate) GetOffset() int {
	return (p.GetPage() - 1) * p.GetSize()
}
