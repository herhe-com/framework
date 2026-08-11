package click

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/golang/freetype/truetype"
	"github.com/spf13/viper"
	"github.com/wenlng/go-captcha/v2/base/option"
	goclick "github.com/wenlng/go-captcha/v2/click"

	"github.com/herhe-com/framework/captcha/internal/resources"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
)

// Click is the click captcha driver.
type Click struct {
	root          string
	minLen        int
	maxLen        int
	width         int
	height        int
	padding       int
	characters    string
	backgroundDir string
	fontDir       string
}

var _ contractcaptcha.Driver = (*Click)(nil)

// NewClick creates a click captcha driver from captcha configuration.
func NewClick(root string, configs map[string]any) *Click {
	cfg := viper.New()
	cfg.Set("captcha", configs)
	cfg.SetDefault("captcha.click.min", 4)
	cfg.SetDefault("captcha.click.max", 4)
	cfg.SetDefault("captcha.click.width", 300)
	cfg.SetDefault("captcha.click.height", 220)
	cfg.SetDefault("captcha.click.padding", 5)

	return &Click{
		root:          root,
		minLen:        cfg.GetInt("captcha.click.min"),
		maxLen:        cfg.GetInt("captcha.click.max"),
		width:         cfg.GetInt("captcha.click.width"),
		height:        cfg.GetInt("captcha.click.height"),
		padding:       cfg.GetInt("captcha.click.padding"),
		characters:    cfg.GetString("captcha.click.char"),
		backgroundDir: cfg.GetString("captcha.resources.bg"),
		fontDir:       cfg.GetString("captcha.resources.font"),
	}
}

// Generate creates a click captcha challenge.
func (r *Click) Generate() (*contractcaptcha.Challenge, error) {
	characters := strings.Split(r.characters, "")
	if r.characters == "" {
		characters = []string{
			"诚", "信", "立", "业", "创", "新", "驱", "动",
			"协", "作", "共", "赢", "服", "务", "社", "会",
			"成", "就", "卓", "越", "未", "来", "责", "任",
			"品", "质", "担", "当", "发", "展", "愿", "景",
		}
	}

	font, err := resources.Font(r.root, r.fontDir)
	if err != nil {
		return nil, err
	}
	backgrounds, err := resources.Backgrounds(r.root, r.backgroundDir)
	if err != nil {
		return nil, err
	}

	builder := goclick.NewBuilder(
		goclick.WithImageSize(option.Size{Width: r.width, Height: r.height}),
		goclick.WithRangeLen(option.RangeVal{Min: r.minLen, Max: r.maxLen}),
		goclick.WithRangeVerifyLen(option.RangeVal{Min: r.minLen, Max: r.maxLen}),
	)
	builder.SetResources(
		goclick.WithChars(characters),
		goclick.WithFonts([]*truetype.Font{font}),
		goclick.WithBackgrounds(backgrounds),
	)

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

// Verify validates click coordinates against a generated target.
func (r *Click) Verify(data contractcaptcha.VerifyData, target json.RawMessage) error {
	var targets map[int]*goclick.Dot
	if err := json.Unmarshal(target, &targets); err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("验证码错误")
	}
	if len(data.Dots) != len(targets) {
		return errors.New("验证码长度不一致")
	}

	for _, expected := range targets {
		if expected == nil {
			return errors.New("验证码错误")
		}

		matched := false
		for _, actual := range data.Dots {
			if expected.Index != actual.Index {
				continue
			}

			matched = true
			if !goclick.Validate(actual.X, actual.Y, expected.X, expected.Y, expected.Width, expected.Height, r.padding) {
				return errors.New("验证码错误")
			}
		}
		if !matched {
			return errors.New("验证码错误")
		}
	}

	return nil
}
