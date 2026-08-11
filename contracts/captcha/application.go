package captcha

import (
	"context"
	"encoding/json"
	"errors"
)

// Application generates and verifies captchas through configured drivers.
type Application interface {
	Generate(ctx context.Context) (*Captcha, error)
	Verify(ctx context.Context, data VerifyData) error
	Driver(driver string) (Driver, error)
}

// Driver generates and verifies one captcha type.
type Driver interface {
	Generate() (*Challenge, error)
	Verify(data VerifyData, target json.RawMessage) error
}

// Challenge contains the public images and private verification target produced by a driver.
type Challenge struct {
	Master string
	Thumb  string
	Target json.RawMessage
}

// Verify validates the user input through the captcha application.
func (r VerifyData) Verify(ctx context.Context, application Application) error {
	if application == nil {
		return errors.New("请先初始化验证码服务")
	}

	return application.Verify(ctx, r)
}
