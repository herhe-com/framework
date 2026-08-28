package slide

import (
	"encoding/json"
	"errors"

	"github.com/spf13/viper"
	"github.com/wenlng/go-captcha/v2/base/option"
	goslide "github.com/wenlng/go-captcha/v2/slide"

	"github.com/herhe-com/framework/captcha/internal/resources"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
)

// Slide is the slide captcha driver.
type Slide struct {
	root          string
	width         int
	height        int
	padding       int
	backgroundDir string
	tileDir       string
}

var _ contractcaptcha.Driver = (*Slide)(nil)

// NewSlide creates a slide captcha driver from captcha configuration.
func NewSlide(root string, configs map[string]any) *Slide {
	cfg := viper.New()
	cfg.Set("captcha", configs)
	cfg.SetDefault("captcha.slide.width", 300)
	cfg.SetDefault("captcha.slide.height", 220)
	cfg.SetDefault("captcha.slide.padding", 5)

	return &Slide{
		root:          root,
		width:         cfg.GetInt("captcha.slide.width"),
		height:        cfg.GetInt("captcha.slide.height"),
		padding:       cfg.GetInt("captcha.slide.padding"),
		backgroundDir: cfg.GetString("captcha.resources.bg"),
		tileDir:       cfg.GetString("captcha.resources.tile"),
	}
}

// Generate creates a slide captcha challenge.
func (r *Slide) Generate() (*contractcaptcha.Challenge, error) {
	backgrounds, err := resources.Backgrounds(r.root, r.backgroundDir)
	if err != nil {
		return nil, err
	}
	tiles, err := resources.Tiles(r.root, r.tileDir)
	if err != nil {
		return nil, err
	}

	builder := goslide.NewBuilder(goslide.WithImageSize(option.Size{Width: r.width, Height: r.height}))
	builder.SetResources(goslide.WithBackgrounds(backgrounds), goslide.WithGraphImages(tiles))
	data, err := builder.Make().Generate()
	if err != nil {
		return nil, err
	}
	block := data.GetData()
	if block == nil {
		return nil, contractcaptcha.ErrCaptchaGenerate
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	thumb, err := data.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}
	target, err := json.Marshal(block)
	if err != nil {
		return nil, err
	}

	return &contractcaptcha.Challenge{Master: master, Thumb: thumb, Target: target, Y: block.DY}, nil
}

// Verify validates the slide position against a generated target.
func (r *Slide) Verify(data contractcaptcha.VerifyData, target json.RawMessage) error {
	var expected *goslide.Block
	if err := json.Unmarshal(target, &expected); err != nil {
		return err
	}
	if expected == nil {
		return errors.New("验证码错误")
	}

	x, y, ok := slideInput(data)
	if !ok || !within(x, expected.X, r.padding) || (y != nil && !within(*y, expected.Y, r.padding)) {
		return errors.New("验证码错误")
	}

	return nil
}

// slideInput extracts the slide coordinates from the user input. Dots is the
// canonical input and carries both coordinates. The top-level X is a
// deprecated fallback that carries no Y (returned as nil) and cannot verify a
// zero coordinate; it will be removed together with VerifyData.X.
func slideInput(data contractcaptcha.VerifyData) (x int, y *int, ok bool) {
	if len(data.Dots) > 0 {
		return data.Dots[0].X, &data.Dots[0].Y, true
	}
	if data.X != 0 {
		return data.X, nil, true
	}
	return 0, nil, false
}

func within(got, want, padding int) bool {
	return got >= want-padding && got <= want+padding
}
