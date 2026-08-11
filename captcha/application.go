package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"

	clickcaptcha "github.com/herhe-com/framework/captcha/click"
	rotatecaptcha "github.com/herhe-com/framework/captcha/rotate"
	slidecaptcha "github.com/herhe-com/framework/captcha/slide"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/support/util"
)

const (
	DriverAuto   = "auto"
	DriverClick  = "click"
	DriverSlide  = "slide"
	DriverRotate = "rotate"
)

var concreteDrivers = []string{DriverClick, DriverSlide, DriverRotate}

type storedCaptcha struct {
	Driver string          `json:"driver"`
	Target json.RawMessage `json:"target"`
}

// Captcha is the captcha application.
type Captcha struct {
	mu      sync.RWMutex
	drivers map[string]contractcaptcha.Driver
}

var _ contractcaptcha.Application = (*Captcha)(nil)

// NewCaptcha creates the captcha application.
func NewCaptcha() *Captcha {
	application, err := NewCaptchaWithError()
	if err != nil {
		color.Errorf("[captcha] %s", err)
		return nil
	}

	return application
}

// NewCaptchaWithError creates the captcha application and returns initialization errors.
func NewCaptchaWithError() (*Captcha, error) {
	application := newCaptcha()
	if !facades.Config().GetBool("captcha.enable", true) {
		return application, nil
	}

	driver := DefaultDriver()
	if driver == DriverAuto {
		return application, nil
	}

	instance, err := NewDriver(driver)
	if err != nil {
		return nil, err
	}
	application.drivers[driver] = instance

	return application, nil
}

// DefaultDriver returns the configured default captcha driver.
func DefaultDriver() string {
	return strings.ToLower(facades.Config().GetString("captcha.driver", DriverClick))
}

// NewDriver creates a captcha driver from the captcha configuration.
func NewDriver(driver string) (contractcaptcha.Driver, error) {
	driver = strings.ToLower(driver)
	configs, _ := facades.Config().Get("captcha").(map[string]any)

	switch driver {
	case DriverClick:
		return clickcaptcha.NewClick(facades.Root(), configs), nil
	case DriverSlide:
		return slidecaptcha.NewSlide(facades.Root(), configs), nil
	case DriverRotate:
		return rotatecaptcha.NewRotate(facades.Root(), configs), nil
	default:
		return nil, fmt.Errorf("invalid captcha driver: %s, only support click, slide, rotate", driver)
	}
}

// Driver returns a cached captcha driver.
func (r *Captcha) Driver(driver string) (contractcaptcha.Driver, error) {
	driver = strings.ToLower(driver)

	r.mu.RLock()
	if instance, ok := r.drivers[driver]; ok {
		r.mu.RUnlock()
		return instance, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if instance, ok := r.drivers[driver]; ok {
		return instance, nil
	}

	instance, err := NewDriver(driver)
	if err != nil {
		return nil, err
	}
	r.drivers[driver] = instance

	return instance, nil
}

// Generate creates the configured captcha and stores its verification target in Redis.
func (r *Captcha) Generate(ctx context.Context) (*contractcaptcha.Captcha, error) {
	if !facades.Config().GetBool("captcha.enable", true) {
		return nil, nil
	}

	driverName := DefaultDriver()
	if driverName == DriverAuto {
		driverName = concreteDrivers[rand.Intn(len(concreteDrivers))]
	}

	driver, err := r.Driver(driverName)
	if err != nil {
		return nil, err
	}
	challenge, err := driver.Generate()
	if err != nil {
		return nil, err
	}
	if challenge == nil || len(challenge.Target) == 0 {
		return nil, contractcaptcha.ErrCaptchaGenerate
	}

	result := &contractcaptcha.Captcha{
		Key:    facades.Snowflake().Generate().String(),
		Driver: driverName,
		Master: challenge.Master,
		Thumb:  challenge.Thumb,
	}
	payload, err := json.Marshal(storedCaptcha{Driver: driverName, Target: challenge.Target})
	if err != nil {
		return nil, err
	}

	expire := facades.Config().GetInt("captcha.expire", 300)
	if expire <= 0 {
		return nil, errors.New("验证码过期时间必须大于 0")
	}
	redis, ok := facades.OptionalRedis()
	if !ok || redis.Default() == nil {
		return nil, errors.New("请先初始化 Redis")
	}
	if err = redis.Default().Set(ctx, captchaRedisKey(result.Key), payload, time.Duration(expire)*time.Second).Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// Verify loads the stored driver and validates the matching user data.
func (r *Captcha) Verify(ctx context.Context, data contractcaptcha.VerifyData) error {
	if !facades.Config().GetBool("captcha.enable", true) {
		return nil
	}

	redisKey := captchaRedisKey(data.Key)
	redis, ok := facades.OptionalRedis()
	if !ok || redis.Default() == nil {
		return errors.New("请先初始化 Redis")
	}
	payload, err := redis.Default().Get(ctx, redisKey).Bytes()
	if err != nil {
		return errors.New("验证码不存在或已过期")
	}

	var stored storedCaptcha
	if err = json.Unmarshal(payload, &stored); err != nil {
		return err
	}
	driver, err := r.Driver(stored.Driver)
	if err != nil {
		return errors.New("验证码类型错误")
	}
	if err = driver.Verify(data, stored.Target); err != nil {
		return err
	}

	return redis.Default().Del(ctx, redisKey).Err()
}

// Generate creates a captcha through the registered application or a standalone application.
func Generate(ctx context.Context) (*contractcaptcha.Captcha, error) {
	if application, ok := facades.Optional[contractcaptcha.Application](); ok {
		return application.Generate(ctx)
	}

	return newCaptcha().Generate(ctx)
}

// Verify validates a captcha through the registered application or a standalone application.
func Verify(ctx context.Context, data contractcaptcha.VerifyData) error {
	if application, ok := facades.Optional[contractcaptcha.Application](); ok {
		return application.Verify(ctx, data)
	}

	return newCaptcha().Verify(ctx, data)
}

func newCaptcha() *Captcha {
	return &Captcha{drivers: make(map[string]contractcaptcha.Driver)}
}

func captchaRedisKey(key string) string {
	return util.Keys("captcha", key)
}
