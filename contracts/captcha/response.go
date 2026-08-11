package captcha

import (
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

type Click struct {
	Master string
	Thumb  string
	Dots   map[int]*click.Dot
}

type Dot struct {
	Index int `json:"index" form:"index" validate:"min=0" label:"索引"`
	X     int `json:"x" form:"x" validate:"min=0" label:"横坐标"`
	Y     int `json:"y" form:"y" validate:"min=0" label:"纵坐标"`
}

// Captcha is the common response returned by Generate.
type Captcha struct {
	Key    string `json:"key"`
	Driver string `json:"driver"`
	Master string `json:"master"`
	Thumb  string `json:"thumb"`
}

// VerifyData contains the form data used to verify a captcha.
// Click uses Dots, slide uses X, and rotate uses Angle.
type VerifyData struct {
	Key   string `json:"key" form:"key" validate:"required" label:"验证码标识"`
	Dots  []Dot  `json:"dots" form:"dots" validate:"omitempty,min=1,dive" label:"点击坐标"`
	X     int    `json:"x" form:"x" validate:"min=0" label:"滑块横坐标"`
	Angle int    `json:"angle" form:"angle" validate:"min=0" label:"旋转角度"`
}

type Slide struct {
	Master string       `json:"master"`
	Thumb  string       `json:"thumb"`
	Block  *slide.Block `json:"-"`
}

type Rotate struct {
	Master string        `json:"master"`
	Thumb  string        `json:"thumb"`
	Block  *rotate.Block `json:"-"`
}
