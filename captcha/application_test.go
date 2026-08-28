package captcha

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	frameworkconfig "github.com/herhe-com/framework/config"
	"github.com/herhe-com/framework/contracts/captcha"
	"github.com/herhe-com/framework/facades"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

func TestDisabledCaptchaBypassesGenerateAndVerify(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}
	facades.Config().Set("captcha.enable", false)

	result, err := Generate(context.Background())
	if err != nil || result != nil {
		t.Fatalf("generate disabled captcha: result=%v, err=%v", result, err)
	}
	if err := Verify(context.Background(), captcha.VerifyData{}); err != nil {
		t.Fatalf("verify disabled captcha: %v", err)
	}
}

func TestSlideChallengeExposesDisplayY(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	driver, err := NewDriver(DriverSlide)
	if err != nil {
		t.Fatalf("create slide driver: %v", err)
	}
	challenge, err := driver.Generate()
	if err != nil {
		t.Fatalf("generate slide captcha: %v", err)
	}

	// The vertical display coordinate must be exposed to the client.
	if challenge.Y <= 0 {
		t.Fatalf("expected slide display Y > 0, got %d", challenge.Y)
	}

	// The horizontal answer X must only live in Target, and the exposed Y must not
	// leak it.
	var block slide.Block
	if err := json.Unmarshal(challenge.Target, &block); err != nil {
		t.Fatalf("unmarshal slide target: %v", err)
	}
	if block.X < 0 {
		t.Fatalf("expected target X >= 0, got %d", block.X)
	}
	if challenge.Y != block.DY {
		t.Fatalf("expected challenge.Y (%d) to match block.DY (%d)", challenge.Y, block.DY)
	}

	if err := SlideVerify(block.X, &slide.Block{X: block.X, Y: 0}); err != nil {
		t.Fatalf("verify generated slide: %v", err)
	}
	if err := SlideVerify(block.X+20, &slide.Block{X: block.X, Y: 0}); err == nil {
		t.Fatal("expected invalid generated slide position")
	}
}

func TestLegacyResultsHideVerificationAnswers(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	// The serialized legacy results must never expose verification answers.
	clickCaptcha, err := Click()
	if err != nil {
		t.Fatalf("generate click captcha: %v", err)
	}
	data, err := json.Marshal(clickCaptcha)
	if err != nil {
		t.Fatalf("marshal click captcha: %v", err)
	}
	if len(clickCaptcha.Dots) == 0 {
		t.Fatal("expected click dots for answer")
	}
	clickJSON := string(data)
	if strings.Contains(clickJSON, "\"index\"") || strings.Contains(clickJSON, "\"x\"") || strings.Contains(clickJSON, "\"text\"") {
		t.Fatalf("click captcha leaked verification answer: %s", clickJSON)
	}

	slideCaptcha, err := Slide()
	if err != nil {
		t.Fatalf("generate slide captcha: %v", err)
	}
	data, err = json.Marshal(slideCaptcha)
	if err != nil {
		t.Fatalf("marshal slide captcha: %v", err)
	}
	slideJSON := string(data)
	if strings.Contains(slideJSON, "\"x\"") || strings.Contains(slideJSON, "\"dx\"") {
		t.Fatalf("slide captcha leaked verification answer: %s", slideJSON)
	}

	rotateCaptcha, err := Rotate()
	if err != nil {
		t.Fatalf("generate rotate captcha: %v", err)
	}
	data, err = json.Marshal(rotateCaptcha)
	if err != nil {
		t.Fatalf("marshal rotate captcha: %v", err)
	}
	rotateJSON := string(data)
	if strings.Contains(rotateJSON, "\"angle\"") {
		t.Fatalf("rotate captcha leaked verification answer: %s", rotateJSON)
	}
}

func TestServiceProviderRegistersCaptchaApplication(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	if err := (&ServiceProvider{}).Register(); err != nil {
		t.Fatalf("register captcha service: %v", err)
	}
	if facades.Captcha() == nil {
		t.Fatal("expected registered captcha application")
	}
}

func TestResourcesFallbackToEmbeddedWhenUnconfigured(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	clickCaptcha, err := Click()
	if err != nil {
		t.Fatalf("generate click captcha: %v", err)
	}
	if clickCaptcha.Master == "" || clickCaptcha.Thumb == "" || len(clickCaptcha.Dots) == 0 {
		t.Fatal("expected complete click captcha")
	}

	slideCaptcha, err := Slide()
	if err != nil {
		t.Fatalf("generate slide captcha: %v", err)
	}
	if slideCaptcha.Master == "" || slideCaptcha.Thumb == "" || slideCaptcha.Block == nil {
		t.Fatal("expected complete slide captcha")
	}

	rotateCaptcha, err := Rotate()
	if err != nil {
		t.Fatalf("generate rotate captcha: %v", err)
	}
	if rotateCaptcha.Master == "" || rotateCaptcha.Thumb == "" || rotateCaptcha.Block == nil {
		t.Fatal("expected complete rotate captcha")
	}
}

func TestNewDriver(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	for _, name := range []string{DriverClick, DriverSlide, DriverRotate} {
		driver, err := NewDriver(name)
		if err != nil {
			t.Fatalf("create %s driver: %v", name, err)
		}
		if driver == nil {
			t.Fatalf("expected %s driver", name)
		}
	}
	if _, err := NewDriver("unsupported"); err == nil {
		t.Fatal("expected unsupported driver error")
	}

	application := newCaptcha()
	first, err := application.Driver(DriverClick)
	if err != nil {
		t.Fatalf("create cached click driver: %v", err)
	}
	second, err := application.Driver(DriverClick)
	if err != nil {
		t.Fatalf("load cached click driver: %v", err)
	}
	if first != second {
		t.Fatal("expected cached captcha driver")
	}
}

func TestSlideAndRotateVerify(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	if err := SlideVerify(100, &slide.Block{X: 100, Y: 0}); err != nil {
		t.Fatalf("verify slide: %v", err)
	}

	driver, err := NewDriver(DriverSlide)
	if err != nil {
		t.Fatalf("create slide driver: %v", err)
	}

	// Dots is the canonical slide input; the Y coordinate is verified as well.
	target, err := json.Marshal(&slide.Block{X: 204, Y: 86})
	if err != nil {
		t.Fatalf("marshal slide target: %v", err)
	}
	if err := driver.Verify(captcha.VerifyData{Dots: []captcha.Dot{{Index: 0, X: 205, Y: 86}}}, target); err != nil {
		t.Fatalf("verify slide from dots: %v", err)
	}
	if err := driver.Verify(captcha.VerifyData{Dots: []captcha.Dot{{Index: 0, X: 205, Y: 120}}}, target); err == nil {
		t.Fatal("expected slide dots with wrong y to fail")
	}

	// A zero x coordinate is a legitimate answer and must verify through Dots.
	zeroTarget, err := json.Marshal(&slide.Block{X: 0, Y: 0})
	if err != nil {
		t.Fatalf("marshal zero slide target: %v", err)
	}
	if err := driver.Verify(captcha.VerifyData{Dots: []captcha.Dot{{X: 0, Y: 0}}}, zeroTarget); err != nil {
		t.Fatalf("verify slide at zero coordinate: %v", err)
	}

	// Missing coordinates and nil targets must fail.
	if err := driver.Verify(captcha.VerifyData{}, target); err == nil {
		t.Fatal("expected missing slide coordinates")
	}
	if err := driver.Verify(captcha.VerifyData{}, json.RawMessage(`{"x":0,"y":0}`)); err == nil {
		t.Fatal("expected missing slide coordinates against zero target")
	}

	// Deprecated top-level X remains accepted for one release cycle.
	if err := driver.Verify(captcha.VerifyData{X: 204}, target); err != nil {
		t.Fatalf("verify slide from legacy x: %v", err)
	}

	if err := RotateVerify(270, &rotate.Block{Angle: 90}); err != nil {
		t.Fatalf("verify rotate: %v", err)
	}
	if err := ClickVerify(
		[]captcha.Dot{{Index: 0, X: 100, Y: 100}},
		[]click.Dot{{Index: 0, X: 100, Y: 100, Width: 20, Height: 20}},
	); err != nil {
		t.Fatalf("verify click: %v", err)
	}
	if err := SlideVerify(120, &slide.Block{X: 100, Y: 0}); err == nil {
		t.Fatal("expected invalid slide position")
	}
	if err := RotateVerify(250, &rotate.Block{Angle: 90}); err == nil {
		t.Fatal("expected invalid rotate angle")
	}
	if err := SlideVerify(0, nil); err == nil {
		t.Fatal("expected nil slide target error")
	}
	if err := RotateVerify(0, nil); err == nil {
		t.Fatal("expected nil rotate target error")
	}
}
