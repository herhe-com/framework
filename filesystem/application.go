package filesystem

import (
	"context"
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/filesystem/minio"
	"github.com/herhe-com/framework/filesystem/qiniu"
)

const (
	DriverS3     string = "s3"
	DriverOss    string = "oss"
	DriverCos    string = "cos"
	DriverMinio  string = "minio"
	DriverQiniu  string = "qiniu"
	DriverCustom string = "custom"
)

type Storage struct {
	filesystem.Driver
	drivers map[string]filesystem.Driver
}

func NewStorage() *Storage {

	defaultDisk := facades.Cfg.GetString("filesystem.driver")

	if defaultDisk == "" {
		color.Redln("[filesystem] please set default disk")
		return nil
	}

	driver, err := NewDriver(defaultDisk)
	if err != nil {
		color.Redf("[filesystem] %s\n", err)

		return nil
	}

	drivers := make(map[string]filesystem.Driver)
	drivers[defaultDisk] = driver

	return &Storage{
		Driver:  driver,
		drivers: drivers,
	}
}

func NewDriver(driver string) (filesystem.Driver, error) {

	ctx := context.Background()

	switch driver {
	//case DriverOss:
	//	return NewOss(ctx, disk)
	//case DriverCos:
	//	return NewCos(ctx, disk)
	//case DriverS3:
	//	return NewS3(ctx, disk)
	case DriverMinio:
		cfg, _ := facades.Cfg.Get("filesystem.minio").(map[string]any)
		return minio.NewMinio(ctx, cfg)
	case DriverQiniu:
		cfg, _ := facades.Cfg.Get("filesystem.qiniu").(map[string]any)
		return qiniu.NewQiniu(ctx, cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support local, minio, qiniu", driver)
}

func (r *Storage) Disk(disk string) filesystem.Driver {

	if driver, exist := r.drivers[disk]; exist {
		return driver
	}

	driver, err := NewDriver(disk)
	if err != nil {
		panic(err)
	}

	r.drivers[disk] = driver

	return driver
}
