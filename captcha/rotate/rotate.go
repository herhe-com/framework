package rotate

import (
	"encoding/json"
	"errors"

	"github.com/spf13/viper"
	gorotate "github.com/wenlng/go-captcha/v2/rotate"

	"github.com/herhe-com/framework/captcha/internal/resources"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
)

// Rotate is the rotate captcha driver.
type Rotate struct {
	root          string
	size          int
	padding       int
	backgroundDir string
}

var _ contractcaptcha.Driver = (*Rotate)(nil)

// NewRotate creates a rotate captcha driver from captcha configuration.
func NewRotate(root string, configs map[string]any) *Rotate {
	cfg := viper.New()
	cfg.Set("captcha", configs)
	cfg.SetDefault("captcha.rotate.size", 220)
	cfg.SetDefault("captcha.rotate.padding", 5)

	return &Rotate{
		root:          root,
		size:          cfg.GetInt("captcha.rotate.size"),
		padding:       cfg.GetInt("captcha.rotate.padding"),
		backgroundDir: cfg.GetString("captcha.resources.bg"),
	}
}

// Generate creates a rotate captcha challenge.
func (r *Rotate) Generate() (*contractcaptcha.Challenge, error) {
	images, err := resources.Backgrounds(r.root, r.backgroundDir)
	if err != nil {
		return nil, err
	}

	builder := gorotate.NewBuilder(gorotate.WithImageSquareSize(r.size))
	builder.SetResources(gorotate.WithImages(images))
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
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, err
	}
	target, err := json.Marshal(data.GetData())
	if err != nil {
		return nil, err
	}

	return &contractcaptcha.Challenge{Master: master, Thumb: thumb, Target: target}, nil
}

// Verify validates the rotation angle against a generated target.
func (r *Rotate) Verify(data contractcaptcha.VerifyData, target json.RawMessage) error {
	var expected *gorotate.Block
	if err := json.Unmarshal(target, &expected); err != nil {
		return err
	}
	if expected == nil || !gorotate.Validate(data.Angle, expected.Angle, r.padding) {
		return errors.New("验证码错误")
	}

	return nil
}
