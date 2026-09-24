package captcha

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	goredis "github.com/redis/go-redis/v9"

	clickcaptcha "github.com/herhe-com/framework/captcha/click"
	rotatecaptcha "github.com/herhe-com/framework/captcha/rotate"
	slidecaptcha "github.com/herhe-com/framework/captcha/slide"
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/support/util"
)

const (
	DriverGoCaptcha = "go-captcha"
	TypeAuto        = "auto"
	TypeClick       = "click"
	TypeSlide       = "slide"
	TypeRotate      = "rotate"
)

var concreteTypes = []string{TypeClick, TypeSlide, TypeRotate}

type storedCaptcha struct {
	Driver string          `json:"driver"`
	Type   string          `json:"type"`
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
	return newCaptcha(), nil
}

// DefaultDriver returns the configured captcha engine for a scene.
func DefaultDriver(scene string) string {
	return strings.ToLower(facades.Config().GetString("captcha.scenes."+scene+".driver", DriverGoCaptcha))
}

// NewDriver creates a captcha type using the configured captcha engine.
func NewDriver(captchaType string) (contractcaptcha.Driver, error) {
	captchaType = strings.ToLower(captchaType)
	configs, _ := facades.Config().Get("captcha").(map[string]any)

	switch captchaType {
	case TypeClick:
		return clickcaptcha.NewClick(facades.Root(), configs), nil
	case TypeSlide:
		return slidecaptcha.NewSlide(facades.Root(), configs), nil
	case TypeRotate:
		return rotatecaptcha.NewRotate(facades.Root(), configs), nil
	default:
		return nil, fmt.Errorf("invalid captcha type: %s, only support click, slide, rotate", captchaType)
	}
}

// Driver returns a cached captcha driver.
func (r *Captcha) Driver(captchaType string) (contractcaptcha.Driver, error) {
	captchaType = strings.ToLower(captchaType)

	r.mu.RLock()
	if instance, ok := r.drivers[captchaType]; ok {
		r.mu.RUnlock()
		return instance, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if instance, ok := r.drivers[captchaType]; ok {
		return instance, nil
	}

	instance, err := NewDriver(captchaType)
	if err != nil {
		return nil, err
	}
	r.drivers[captchaType] = instance

	return instance, nil
}

// Generate creates the configured captcha and stores its verification target in Redis.
func (r *Captcha) Generate(ctx context.Context, scene string) (*contractcaptcha.Captcha, error) {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, errors.New("验证码场景不能为空")
	}
	enabled, err := captchaSceneEnabled(scene)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, nil
	}
	driverName := DefaultDriver(scene)
	if driverName != DriverGoCaptcha {
		return nil, fmt.Errorf("invalid captcha driver: %s, only support %s", driverName, DriverGoCaptcha)
	}

	captchaType := strings.ToLower(facades.Config().GetString("captcha.scenes."+scene+".type", TypeClick))
	if captchaType == TypeAuto {
		captchaType = concreteTypes[rand.Intn(len(concreteTypes))]
	}

	driver, err := r.Driver(captchaType)
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
		Type:   captchaType,
		Master: challenge.Master,
		Thumb:  challenge.Thumb,
		Y:      challenge.Y,
	}
	payload, err := json.Marshal(storedCaptcha{Driver: driverName, Type: captchaType, Target: challenge.Target})
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
	if err = redis.Default().Set(ctx, captchaRedisKey(scene, result.Key), payload, time.Duration(expire)*time.Second).Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// Verify validates the matching user data without consuming the captcha.
func (r *Captcha) Verify(ctx context.Context, data contractcaptcha.VerifyData) error {
	return r.verify(ctx, data, false)
}

// Use validates and consumes a captcha after the surrounding action succeeds.
func (r *Captcha) Use(ctx context.Context, data contractcaptcha.VerifyData) error {
	return r.verify(ctx, data, true)
}

func (r *Captcha) verify(ctx context.Context, data contractcaptcha.VerifyData, consume bool) error {
	data.Scene = strings.TrimSpace(data.Scene)
	if data.Scene == "" {
		return errors.New("验证码场景不能为空")
	}
	data.Key = strings.TrimSpace(data.Key)
	enabled, err := captchaSceneEnabled(data.Scene)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if data.Key == "" {
		return errors.New("验证码标识不能为空")
	}

	redisKey := captchaRedisKey(data.Scene, data.Key)
	redis, ok := facades.OptionalRedis()
	if !ok || redis.Default() == nil {
		return errors.New("请先初始化 Redis")
	}
	payload, err := redis.Default().Get(ctx, redisKey).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return errors.New("验证码不存在或已过期")
		}
		return err
	}

	var stored storedCaptcha
	if err = json.Unmarshal(payload, &stored); err != nil {
		return err
	}
	// Accept captcha records written before engine and type were stored separately.
	if stored.Type == "" {
		stored.Type = stored.Driver
		stored.Driver = DriverGoCaptcha
	}
	if stored.Driver != DriverGoCaptcha {
		return errors.New("验证码引擎类型错误")
	}
	driver, err := r.Driver(stored.Type)
	if err != nil {
		return errors.New("验证码类型错误")
	}
	if err = driver.Verify(data, stored.Target); err != nil {
		return err
	}

	if consume {
		deleted, err := redis.Default().Del(ctx, redisKey).Result()
		if err != nil {
			return err
		}
		if deleted != 1 {
			return errors.New("验证码不存在或已过期")
		}
	}
	return nil
}

// Generate creates a captcha through the registered application or a standalone application.
func Generate(ctx context.Context, scene string) (*contractcaptcha.Captcha, error) {
	if application, ok := facades.Optional[contractcaptcha.Application](); ok {
		return application.Generate(ctx, scene)
	}

	return newCaptcha().Generate(ctx, scene)
}

// Verify validates a captcha through the registered application or a standalone application.
func Verify(ctx context.Context, data contractcaptcha.VerifyData) error {
	if application, ok := facades.Optional[contractcaptcha.Application](); ok {
		return application.Verify(ctx, data)
	}

	return newCaptcha().Verify(ctx, data)
}

// Use validates and consumes a captcha through the registered application or a standalone application.
func Use(ctx context.Context, data contractcaptcha.VerifyData) error {
	if application, ok := facades.Optional[contractcaptcha.Application](); ok {
		return application.Use(ctx, data)
	}

	return newCaptcha().Use(ctx, data)
}

func newCaptcha() *Captcha {
	return &Captcha{drivers: make(map[string]contractcaptcha.Driver)}
}

func captchaRedisKey(scene, key string) string {
	return util.Keys("captcha", base64.RawURLEncoding.EncodeToString([]byte(scene)), key)
}

func captchaSceneEnabled(scene string) (bool, error) {
	key := "captcha.scenes." + scene + ".enable"
	if !facades.Config().IsSet(key) {
		return false, fmt.Errorf("验证码场景未配置: %s", scene)
	}

	return facades.Config().GetBool(key), nil
}
