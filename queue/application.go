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

// Consumer prepares and handles messages for a configured queue key.
type Consumer = queue.Consumer

// Headers contains transport-neutral message headers.
type Headers = queue.Headers

// ErrDisabled indicates that a queue is disabled by configuration.
var ErrDisabled = queue.ErrDisabled

type Queue struct {
	mu      sync.RWMutex
	drivers map[string]queue.Driver
}

var _ queue.Queue = (*Queue)(nil)

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
	return &Queue{
		drivers: make(map[string]queue.Driver),
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
		return rabbitmq.NewRabbitMQ(cfg, name)
	case DriverNATS:
		return natsqueue.NewNATS(cfg, name)
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

// Producer publishes a message using queue.queues.<key>.
func (r *Queue) Producer(body []byte, key string, headers ...queue.Headers) error {
	driver, err := r.driverForQueue(key)
	if err != nil {
		return err
	}

	return driver.Producer(body, key, headers...)
}

// Consumer consumes messages using queue.queues.<key>.
func (r *Queue) Consumer(handler queue.Handler, key string) error {
	driver, err := r.driverForQueue(key)
	if err != nil {
		return err
	}

	return driver.Consumer(handler, key)
}

func (r *Queue) driverForQueue(key string) (queue.Driver, error) {
	definition, err := queueconfig.Load(key)
	if err != nil {
		return nil, err
	}
	if err = definition.EnsureEnabled(); err != nil {
		return nil, err
	}

	return r.Channel(definition.Connection)
}

// Close closes every initialized queue connection.
func (r *Queue) Close() error {
	r.mu.RLock()
	drivers := make([]queue.Driver, 0, len(r.drivers))
	for _, driver := range r.drivers {
		drivers = append(drivers, driver)
	}
	r.mu.RUnlock()

	var firstErr error
	for _, driver := range drivers {
		if err := driver.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
