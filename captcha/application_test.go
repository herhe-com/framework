package captcha

import (
	"context"
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

	if err := SlideVerify(100, &slide.Block{X: 100, Y: 50}); err != nil {
		t.Fatalf("verify slide: %v", err)
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
	if err := SlideVerify(120, &slide.Block{X: 100, Y: 50}); err == nil {
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
