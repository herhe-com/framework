package request

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"

	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
)

type fakeCaptchaApplication struct {
	contractcaptcha.Application
	data contractcaptcha.VerifyData
	err  error
}

func (f *fakeCaptchaApplication) Verify(_ context.Context, data contractcaptcha.VerifyData) error {
	f.data = data
	return f.err
}

func TestCaptchaVerify(t *testing.T) {
	expectedErr := errors.New("invalid captcha")
	application := &fakeCaptchaApplication{err: expectedErr}
	input := Captcha{
		Key:   "captcha-key",
		Dots:  []contractcaptcha.Dot{{Index: 1, X: 10, Y: 20}},
		X:     30,
		Angle: 40,
	}

	err := input.Verify(context.Background(), application)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("verify captcha: %v", err)
	}
	if !reflect.DeepEqual(application.data, input) {
		t.Fatalf("expected data %#v, got %#v", input, application.data)
	}

	if err := input.Verify(context.Background(), nil); err == nil {
		t.Fatal("expected missing captcha application error")
	}
}

func TestCaptchaValidation(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())

	if err := validate.Struct(Captcha{Key: "captcha-key"}); err != nil {
		t.Fatalf("validate captcha: %v", err)
	}
	if err := validate.Struct(Captcha{}); err != nil {
		t.Fatalf("validate empty captcha: %v", err)
	}
	if err := validate.Struct(Captcha{Key: "captcha-key", X: -1}); err == nil {
		t.Fatal("expected invalid captcha position error")
	}
}
