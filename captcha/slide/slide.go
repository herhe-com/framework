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
	if data.GetData() == nil {
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
	target, err := json.Marshal(data.GetData())
	if err != nil {
		return nil, err
	}

	return &contractcaptcha.Challenge{Master: master, Thumb: thumb, Target: target}, nil
}

// Verify validates the slide position against a generated target.
func (r *Slide) Verify(data contractcaptcha.VerifyData, target json.RawMessage) error {
	var expected *goslide.Block
	if err := json.Unmarshal(target, &expected); err != nil {
		return err
	}
	if expected == nil || !goslide.Validate(data.X, expected.Y, expected.X, expected.Y, r.padding) {
		return errors.New("验证码错误")
	}

	return nil
}
