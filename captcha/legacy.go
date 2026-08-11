package captcha

import (
	"encoding/json"

	goclick "github.com/wenlng/go-captcha/v2/click"
	gorotate "github.com/wenlng/go-captcha/v2/rotate"
	goslide "github.com/wenlng/go-captcha/v2/slide"

	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
)

// Click generates a click captcha without storing it in Redis.
func Click() (*contractcaptcha.Click, error) {
	driver, err := NewDriver(DriverClick)
	if err != nil {
		return nil, err
	}
	challenge, err := driver.Generate()
	if err != nil {
		return nil, err
	}

	var dots map[int]*goclick.Dot
	if err = json.Unmarshal(challenge.Target, &dots); err != nil {
		return nil, err
	}

	return &contractcaptcha.Click{Master: challenge.Master, Thumb: challenge.Thumb, Dots: dots}, nil
}

// ClickVerify validates click coordinates without reading Redis.
func ClickVerify(sources []contractcaptcha.Dot, targets []goclick.Dot) error {
	stored := make(map[int]*goclick.Dot, len(targets))
	for i := range targets {
		target := targets[i]
		stored[target.Index] = &target
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	driver, err := NewDriver(DriverClick)
	if err != nil {
		return err
	}

	return driver.Verify(contractcaptcha.VerifyData{Dots: sources}, payload)
}

// Slide generates a slide captcha without storing it in Redis.
func Slide() (*contractcaptcha.Slide, error) {
	driver, err := NewDriver(DriverSlide)
	if err != nil {
		return nil, err
	}
	challenge, err := driver.Generate()
	if err != nil {
		return nil, err
	}

	var block goslide.Block
	if err = json.Unmarshal(challenge.Target, &block); err != nil {
		return nil, err
	}

	return &contractcaptcha.Slide{Master: challenge.Master, Thumb: challenge.Thumb, Block: &block}, nil
}

// SlideVerify validates a slide position without reading Redis.
func SlideVerify(x int, target *goslide.Block) error {
	payload, err := json.Marshal(target)
	if err != nil {
		return err
	}
	driver, err := NewDriver(DriverSlide)
	if err != nil {
		return err
	}

	return driver.Verify(contractcaptcha.VerifyData{X: x}, payload)
}

// Rotate generates a rotate captcha without storing it in Redis.
func Rotate() (*contractcaptcha.Rotate, error) {
	driver, err := NewDriver(DriverRotate)
	if err != nil {
		return nil, err
	}
	challenge, err := driver.Generate()
	if err != nil {
		return nil, err
	}

	var block gorotate.Block
	if err = json.Unmarshal(challenge.Target, &block); err != nil {
		return nil, err
	}

	return &contractcaptcha.Rotate{Master: challenge.Master, Thumb: challenge.Thumb, Block: &block}, nil
}

// RotateVerify validates a rotation angle without reading Redis.
func RotateVerify(angle int, target *gorotate.Block) error {
	payload, err := json.Marshal(target)
	if err != nil {
		return err
	}
	driver, err := NewDriver(DriverRotate)
	if err != nil {
		return err
	}

	return driver.Verify(contractcaptcha.VerifyData{Angle: angle}, payload)
}
