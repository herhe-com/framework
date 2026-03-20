package filesystem

import (
	"context"
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/filesystem/cos"
	"github.com/herhe-com/framework/filesystem/minio"
	"github.com/herhe-com/framework/filesystem/oss"
	"github.com/herhe-com/framework/filesystem/qiniu"
	"github.com/herhe-com/framework/filesystem/s3"
)

const (
	DriverS3    string = "s3"
	DriverOss   string = "oss"
	DriverCos   string = "cos"
	DriverMinio string = "minio"
	DriverQiniu string = "qiniu"
)

type Storage struct {
	filesystem.Driver
	drivers map[string]filesystem.Driver
}

func NewStorage() *Storage {

	defaultDriver := facades.Cfg.GetString("filesystem.driver")

	if defaultDriver == "" {
		color.Redln("[filesystem] please set default driver")
		return nil
	}

	driver, err := NewDriver(defaultDriver, "default")

	if err != nil {
		color.Redf("[filesystem] %s\n", err)
		return nil
	}

	drivers := make(map[string]filesystem.Driver)
	key := fmt.Sprintf("%s_%s", defaultDriver, "default")
	drivers[key] = driver

	return &Storage{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(driver string, disk string) (filesystem.Driver, error) {

	ctx := context.Background()
	configKey := fmt.Sprintf("filesystem.disks.%s", disk)
	cfg, _ := facades.Cfg.Get(configKey).(map[string]any)

	switch driver {
	case DriverOss:
		return oss.NewOSS(ctx, cfg)
	case DriverCos:
		return cos.NewCOS(ctx, cfg)
	case DriverS3:
		return s3.NewS3(ctx, cfg)
	case DriverMinio:
		return minio.NewMinio(ctx, cfg)
	case DriverQiniu:
		return qiniu.NewQiniu(ctx, cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support oss, cos, s3, minio, qiniu", driver)
}

func (r *Storage) Disk(driver string, disk string) (filesystem.Driver, error) {

	key := fmt.Sprintf("%s_%s", driver, disk)

	if dri, exist := r.drivers[key]; exist {
		return dri, nil
	}

	dri, err := NewDriver(driver, disk)
	if err != nil {
		return nil, err
	}

	r.drivers[key] = dri

	return dri, nil
}
