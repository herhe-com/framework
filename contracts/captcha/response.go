package captcha

import (
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

// Click is the standalone click captcha result used by the legacy Click helper.
//
// It carries the verification answer inside Dots (coordinates, index and text),
// so those are tagged json:"-" and must never be returned to a client. Only
// Master and Thumb are safe to expose when rendering this result.
type Click struct {
	Master string             `json:"master"`
	Thumb  string             `json:"thumb"`
	Dots   map[int]*click.Dot `json:"-"`
}

type Dot struct {
	Index int `json:"index" form:"index" validate:"min=0" label:"索引"`
	X     int `json:"x" form:"x" validate:"min=0" label:"横坐标"`
	Y     int `json:"y" form:"y" validate:"min=0" label:"纵坐标"`
}

// Captcha is the common response returned by Generate.
//
// Y is the vertical display coordinate for slide captchas, telling the client
// where to align the slider to the gap. It is zero for other drivers. The
// horizontal answer X is never included here; it is kept inside the Redis target.
type Captcha struct {
	Key    string `json:"key"`
	Driver string `json:"driver"`
	Master string `json:"master"`
	Thumb  string `json:"thumb"`
	Y      int    `json:"y,omitempty"`
}

// VerifyData contains the form data used to verify a captcha.
// Click uses Dots, slide uses Dots (a single entry), and rotate uses Angle.
//
// Deprecated: X is the legacy slide input kept for one release cycle. It cannot
// verify a zero coordinate; use Dots instead.
type VerifyData struct {
	Key   string `json:"key" form:"key" validate:"omitempty" label:"验证码标识"`
	Dots  []Dot  `json:"dots" form:"dots" validate:"omitempty,min=1,dive" label:"点击坐标"`
	X     int    `json:"x" form:"x" validate:"min=0" label:"滑块横坐标"`
	Angle int    `json:"angle" form:"angle" validate:"min=0" label:"旋转角度"`
}

// Slide is the standalone slide captcha result used by the legacy Slide helper.
// It carries the verification answer (X coordinate) inside Block, hidden from
// clients via json:"-". When a client needs the vertical gap position, generate
// through captcha.Generate and read Captcha.Y instead.
type Slide struct {
	Master string       `json:"master"`
	Thumb  string       `json:"thumb"`
	Block  *slide.Block `json:"-"`
}

// Rotate is the standalone rotate captcha result used by the legacy Rotate helper.
// It carries the verification answer (rotation angle) inside Block, hidden from
// clients via json:"-".
type Rotate struct {
	Master string        `json:"master"`
	Thumb  string        `json:"thumb"`
	Block  *rotate.Block `json:"-"`
}
