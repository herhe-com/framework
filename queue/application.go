package queue

import (
	"fmt"
	"sync"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	queueconfig "github.com/herhe-com/framework/queue/config"
	natsqueue "github.com/herhe-com/framework/queue/nats"
	"github.com/herhe-com/framework/queue/rabbitmq"
)

const (
	DriverRabbitmq string = "rabbitmq"
	DriverNATS     string = "nats"
)

// Handler processes a queue message body.
type Handler = queue.Handler

// Headers contains transport-neutral message headers.
type Headers = queue.Headers

// ProducerOptions describes where and how a message is published.
type ProducerOptions = queue.ProducerOptions

// ConsumerOptions describes a queue subscription.
type ConsumerOptions = queue.ConsumerOptions

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
	driver, err := NewDriver(defaultName)
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

// NewDriver creates a queue driver from the given connection's configuration.
func NewDriver(name string) (queue.Driver, error) {
	configKey := fmt.Sprintf("queue.connections.%s", name)
	cfg, _ := facades.Config().Get(configKey).(map[string]any)

	driver, ok := cfg["driver"].(string)
	if !ok || driver == "" {
		return nil, fmt.Errorf("please set driver for connection: %s", name)
	}

	switch driver {
	case DriverRabbitmq:
		return rabbitmq.NewRabbitMQ(cfg)
	case DriverNATS:
		return natsqueue.NewNATS(cfg)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support RabbitMQ and NATS", driver)
}

func (r *Queue) Channel(name string) (queue.Driver, error) {
	r.mu.RLock()
	if dri, exist := r.drivers[name]; exist {
		r.mu.RUnlock()
		return dri, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if dri, exist := r.drivers[name]; exist {
		return dri, nil
	}

	dri, err := NewDriver(name)
	if err != nil {
		return nil, err
	}

	r.drivers[name] = dri

	return dri, nil
}
