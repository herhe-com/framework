package captcha

import (
	"errors"
	"image"
	"math/rand"
	"os"
	"path"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/herhe-com/framework/contracts/captcha"
	"github.com/herhe-com/framework/facades"
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha/v2/base/codec"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/click"
)

func Click() (result *captcha.Click, err error) {

	minLen := facades.Cfg.GetInt("captcha.click.min", 4)
	maxLen := facades.Cfg.GetInt("captcha.click.max", 4)

	width := facades.Cfg.GetInt("captcha.click.width", 300)
	height := facades.Cfg.GetInt("captcha.click.height", 220)

	char := facades.Cfg.GetString("captcha.click.char", "")

	var chars []string

	if char != "" {
		chars = strings.Split(char, "")
	} else {
		chars = []string{
			"诚", "信", "立", "业", "创", "新", "驱", "动",
			"协", "作", "共", "赢", "服", "务", "社", "会",
			"成", "就", "卓", "越", "未", "来", "责", "任",
			"品", "质", "担", "当", "发", "展", "愿", "景",
		}
	}

	builder := click.NewBuilder(
		click.WithImageSize(option.Size{
			Width:  width,
			Height: height,
		}),
		click.WithRangeLen(option.RangeVal{Min: minLen, Max: maxLen + 2}),
		click.WithRangeVerifyLen(option.RangeVal{Min: minLen, Max: maxLen}),
	)

	fontN, err := font()

	if err != nil {
		return nil, err
	}

	bgImage, err := background()

	if err != nil {
		return nil, err
	}

	builder.SetResources(
		click.WithChars(chars),
		click.WithFonts([]*truetype.Font{
			fontN,
		}),
		click.WithBackgrounds(bgImage),
	)

	textCapt := builder.Make()

	captData, err := textCapt.Generate()

	if err != nil {
		return nil, err
	}

	result = &captcha.Click{}

	result.Dots = captData.GetData()

	if result.Dots == nil {
		return nil, captcha.ErrCaptchaGenerate
	}

	result.Master, err = captData.GetMasterImage().ToBase64()

	if err != nil {
		return nil, err
	}

	result.Thumb, err = captData.GetThumbImage().ToBase64()

	if err != nil {
		return nil, err
	}

	return result, nil
}

func ClickVerify(sources []captcha.Dot, targets []click.Dot) error {

	padding := facades.Cfg.GetInt("captcha.click.padding", 5)

	if len(sources) != len(targets) {
		return errors.New("验证码长度不一致")
	}

	for _, value := range targets {

		mark := true

		for _, val := range sources {

			if value.Index == val.Index {
				mark = false

				if ok := click.Validate(val.X, val.Y, value.X, value.Y, value.Width, value.Height, padding); !ok {
					return errors.New("验证码错误")
				}
			}
		}

		if mark {
			return errors.New("验证码错误")
		}
	}

	return nil
}

func background() ([]image.Image, error) {

	dir := facades.Cfg.GetString("captcha.resources.bg", "/resources/bg")

	dir = "/" + strings.Trim(dir, "/")

	entries, err := os.ReadDir(facades.Root + dir)

	if err != nil {
		return nil, err
	}

	files := make([]string, 0)

	for _, entry := range entries {

		ext := path.Ext(entry.Name())

		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			files = append(files, entry.Name())
		}
	}

	if len(files) == 0 {
		return imagesv2.GetImages()
	}

	images := make([]image.Image, 0)

	for _, file := range files {

		imgBytes, err := os.ReadFile(facades.Root + dir + "/" + file)

		if err != nil {
			return nil, err
		}

		ext := path.Ext(file)

		switch ext {
		case ".png":
			if img, err := codec.DecodeByteToPng(imgBytes); err == nil {
				images = append(images, img)
			}
		case ".jpg":
			if img, err := codec.DecodeByteToJpeg(imgBytes); err == nil {
				images = append(images, img)
			}
		case ".jpeg":
			if img, err := codec.DecodeByteToJpeg(imgBytes); err == nil {
				images = append(images, img)
			}
		}
	}

	return images, nil
}

func font() (*truetype.Font, error) {

	dir := facades.Cfg.GetString("captcha.resources.font", "/resources/font")

	dir = "/" + strings.Trim(dir, "/")

	entries, _ := os.ReadDir(facades.Root + dir)

	files := make([]string, 0)

	for _, entry := range entries {

		ext := path.Ext(entry.Name())

		if ext == ".ttf" {
			files = append(files, entry.Name())
		}
	}

	if len(files) == 0 {
		return fzshengsksjw.GetFont()
	}

	ft := files[rand.Intn(len(files))]

	fontBytes, err := os.ReadFile(facades.Root + dir + "/" + ft)

	if err != nil {
		return nil, err
	}

	return freetype.ParseFont(fontBytes)
}
