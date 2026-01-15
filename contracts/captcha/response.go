package captcha

import "github.com/wenlng/go-captcha/v2/click"

type Click struct {
	Master string
	Thumb  string
	Dots   map[int]*click.Dot
}

type Dot struct {
	Index int
	X     int
	Y     int
}
