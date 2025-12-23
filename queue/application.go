package queue

import (
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/queue/rabbitmq"
)

const (
	DriverRabbitmq string = "rabbitmq"
)

type Queue struct {
	queue.Driver
	drivers map[string]queue.Driver
}

func NewQueue() *Queue {

	defaultDriver := facades.Cfg.GetString("queue.driver")
	if defaultDriver == "" {
		color.Errorln("[queue] please set default driver")
		return nil
	}

	driver, err := NewDriver(defaultDriver, "default")
	if err != nil {
		color.Errorf("[queue] %s", err)
		return nil
	}

	drivers := make(map[string]queue.Driver)
	drivers[defaultDriver] = driver

	return &Queue{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(driver string, name string) (queue.Driver, error) {

	switch driver {
	case DriverRabbitmq:

		cfg, _ := facades.Cfg.Get("queue.rabbitmq." + name).(map[string]any)

		return rabbitmq.NewRabbitMQ(cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support RabbitMQ", driver)
}

func (r *Queue) Channel(driver string, name string) (queue.Driver, error) {

	key := fmt.Sprintf("%s_%s", driver, name)

	if dri, exist := r.drivers[key]; exist {
		return dri, nil
	}

	dri, err := NewDriver(driver, name)
	if err != nil {
		return nil, err
	}

	r.drivers[key] = dri

	return dri, nil
}
