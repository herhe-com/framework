package queue

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/queue/rabbitmq"
)

type Driver string

const (
	DriverRabbitmq Driver = "rabbitmq"
)

type Queue struct {
	queue.Driver
	drivers map[string]queue.Driver
}

func NewQueue() *Queue {

	defaultChannel := facades.Cfg.GetString("queue.driver")
	if defaultChannel == "" {
		color.Redln("[queue] please set default driver")
		return nil
	}

	driver, err := NewDriver(defaultChannel)
	if err != nil {
		color.Redf("[queue] %s\n", err)

		return nil
	}

	drivers := make(map[string]queue.Driver)
	drivers[defaultChannel] = driver

	return &Queue{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(dri string) (queue.Driver, error) {

	driver := Driver(dri)

	switch driver {
	case DriverRabbitmq:
		cfg, _ := facades.Cfg.Get("queue.rabbitmq").(map[string]any)
		return rabbitmq.NewRabbitMQ(cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support local, minio, qiniu", driver)
}

func (r *Queue) Channel(dri string) (queue.Driver, error) {

	if driver, exist := r.drivers[dri]; exist {
		return driver, nil
	}

	driver, err := NewDriver(dri)
	if err != nil {
		return nil, err
	}

	r.drivers[dri] = driver

	return driver, nil
}
