package request

type Paginate struct {
	Page int `form:"page" json:"page" query:"page" validate:"omitempty,number,gte=0" label:"页码"`
	Size int `form:"size" json:"size" query:"size" validate:"omitempty,number,gte=0,lte=20" label:"页数"`
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
