package queue

import (
	"fmt"
	"sync"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	queueconfig "github.com/herhe-com/framework/queue/config"
	"github.com/herhe-com/framework/queue/rabbitmq"
)

const (
	DriverRabbitmq string = "rabbitmq"
)

type Queue struct {
	queue.Driver
	mu      sync.RWMutex
	drivers map[string]queue.Driver
}

func NewQueue() *Queue {
	queue, err := NewQueueWithError()
	if err != nil {
		color.Errorf("[queue] %s", err)
		return nil
	}

	return queue
}

// NewQueueWithError creates the queue application and returns initialization errors.
func NewQueueWithError() (*Queue, error) {
	defaultName := DefaultName()
	driver, err := NewDriver("", defaultName)
	if err != nil {
		return nil, err
	}

	drivers := make(map[string]queue.Driver)
	drivers[defaultName] = driver

	return &Queue{
		drivers: drivers,
		Driver:  driver,
	}, nil
}

// DefaultName returns the configured default queue connection name.
func DefaultName() string {
	return queueconfig.DefaultName()
}

func NewDriver(driver string, name string) (queue.Driver, error) {
	cfg, _ := facades.Cfg.Get("queue.connections." + name).(map[string]any)
	if len(cfg) == 0 {
		cfg, _ = facades.Cfg.Get("queue.rabbitmq." + name).(map[string]any)
	}
	if driver == "" {
		driver = queueconfig.Driver(name, "")
	}
	if driver == "" {
		driver = DriverRabbitmq
	}

	switch driver {
	case DriverRabbitmq:
		return rabbitmq.NewRabbitMQ(cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support RabbitMQ", driver)
}

func (r *Queue) Channel(driver string, name string) (queue.Driver, error) {

	key := name

	r.mu.RLock()
	if dri, exist := r.drivers[key]; exist {
		r.mu.RUnlock()
		return dri, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

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
