package consoles

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	contractqueue "github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	frameworkqueue "github.com/herhe-com/framework/queue"
	queueconfig "github.com/herhe-com/framework/queue/config"
	"github.com/spf13/cobra"
)

type ConsumerProvider struct {
}

// Reconnect backoff of the consumer supervisor.
var (
	consumerReconnectMin = time.Second
	consumerReconnectMax = 30 * time.Second
)

// consumerDriver pins one driver for the consumer goroutine and lets the
// supervisor replace it after a fatal connection error.
type consumerDriver struct {
	mu     sync.Mutex
	driver contractqueue.Driver
	closed bool
	done   chan struct{}
}

func newConsumerDriver(driver contractqueue.Driver) *consumerDriver {
	return &consumerDriver{driver: driver, done: make(chan struct{})}
}

func (r *consumerDriver) Producer(body []byte, key string, headers ...contractqueue.Headers) error {
	r.mu.Lock()
	driver, closed := r.driver, r.closed
	r.mu.Unlock()

	if closed || driver == nil {
		return fmt.Errorf("consumer driver is closed")
	}

	return driver.Producer(body, key, headers...)
}

func (r *consumerDriver) Consumer(handler contractqueue.Handler, key string) error {
	return r.consume(handler, key)
}

func (r *consumerDriver) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	if r.done != nil {
		close(r.done)
	}
	driver := r.driver
	r.driver = nil
	r.mu.Unlock()

	if driver == nil {
		return nil
	}

	return driver.Close()
}

func (r *consumerDriver) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.closed
}

// closeCurrent detaches and closes the current underlying driver immediately,
// so resources are not held open while waiting for reconnect backoff.
func (r *consumerDriver) closeCurrent() {
	r.mu.Lock()
	driver := r.driver
	r.driver = nil
	r.mu.Unlock()

	if driver != nil {
		_ = driver.Close()
	}
}

// consume blocks on the current driver until it stops.
func (r *consumerDriver) consume(handler contractqueue.Handler, key string) error {
	r.mu.Lock()
	driver, closed := r.driver, r.closed
	r.mu.Unlock()

	if closed || driver == nil {
		return nil
	}

	return driver.Consumer(handler, key)
}

// reconnect rebuilds the driver with exponential backoff until it succeeds or
// the consumer is closed. It reports whether a fresh driver is installed.
func (r *consumerDriver) reconnect(
	consumer contractqueue.Consumer,
	newDriver func(string) (contractqueue.Driver, error),
	connection, key string,
) bool {
	backoff := consumerReconnectMin

	for {
		if !r.wait(backoff) {
			return false
		}

		driver, err := newDriver(connection)
		if err == nil {
			err = consumer.Prepare()
		}
		if err != nil {
			if driver != nil {
				_ = driver.Close()
			}
			color.Errorf("[queue] %s: reconnect failed: %v\n", key, err)
			if backoff *= 2; backoff > consumerReconnectMax {
				backoff = consumerReconnectMax
			}

			continue
		}

		previous, swapped := r.swap(driver)
		if !swapped {
			_ = driver.Close()
			return false
		}
		if previous != nil {
			_ = previous.Close()
		}

		color.Infof("[queue] %s: reconnected to %s\n", key, connection)

		return true
	}
}

func (r *consumerDriver) swap(driver contractqueue.Driver) (contractqueue.Driver, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, false
	}
	previous := r.driver
	r.driver = driver

	return previous, true
}

// wait sleeps for the backoff and reports whether the consumer is still open.
func (r *consumerDriver) wait(d time.Duration) bool {
	r.mu.Lock()
	done, closed := r.done, r.closed
	r.mu.Unlock()

	if closed {
		return false
	}
	if done == nil {
		time.Sleep(d)
		return true
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-done:
		return false
	case <-timer.C:
		return true
	}
}

func (that *ConsumerProvider) Register() console.Console {

	return console.Console{
		Cmd:  "consumer",
		Name: "消费队列",
		Run: func(cmd *cobra.Command, args []string) {
			consumers, ok := facades.Config().Get("queue.consumes").([]contractqueue.Consumer)
			if !ok {
				color.Errorln("[queue] queue.consumes must be []queue.Consumer")
				return
			}
			drivers := make([]contractqueue.Driver, 0, len(consumers))
			var wait sync.WaitGroup

			for _, consumer := range consumers {
				driver, err := that.startConsumer(consumer, &wait, frameworkqueue.NewDriver)
				if err != nil {
					color.Errorf("[queue] consumer: %v\n", err)
					continue
				}

				drivers = append(drivers, driver)
			}

			color.Successf("\n\n消费队列运行成功\n\n")

			signs := make(chan os.Signal, 1)
			signal.Notify(signs, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(signs)

			<-signs

			for _, driver := range drivers {
				if err := driver.Close(); err != nil {
					color.Errorf("[queue] close consumer: %v\n", err)
				}
			}
			wait.Wait()
			color.Warnln("\n\n消费队列已停止运行\n\n")
		},
	}
}

func (that *ConsumerProvider) startConsumer(
	consumer contractqueue.Consumer,
	wait *sync.WaitGroup,
	newDriver func(string) (contractqueue.Driver, error),
) (contractqueue.Driver, error) {
	if wait == nil {
		return nil, fmt.Errorf("consumer wait group is required")
	}
	if newDriver == nil {
		return nil, fmt.Errorf("queue driver factory is required")
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer is required")
	}

	key := strings.TrimSpace(consumer.Key())
	if key == "" {
		return nil, fmt.Errorf("consumer key is required")
	}

	definition, err := queueconfig.Load(key)
	if err != nil {
		return nil, err
	}
	if err = definition.EnsureEnabled(); err != nil {
		return nil, err
	}

	rawDriver, err := newDriver(definition.Connection)
	if err != nil {
		return nil, fmt.Errorf("%s connection %s: %w", key, definition.Connection, err)
	}
	driver := newConsumerDriver(rawDriver)
	if err = consumer.Prepare(); err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("%s prepare: %w", key, err)
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		defer func() { _ = driver.Close() }()

		// A lost connection ends Consumer: rebuild the driver instead of leaving
		// the process running without a consumer.
		for {
			consumeErr := driver.consume(consumer.Handle, key)
			if consumeErr == nil || driver.isClosed() {
				return
			}

			driver.closeCurrent()
			color.Errorf("[queue] %s: %v, reconnecting\n", key, consumeErr)
			if !driver.reconnect(consumer, newDriver, definition.Connection, key) {
				return
			}
		}
	}()

	return driver, nil
}
