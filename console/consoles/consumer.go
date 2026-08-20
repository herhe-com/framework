package consoles

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

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

type consumerDriver struct {
	contractqueue.Driver
	closeOnce sync.Once
	closeErr  error
}

func (r *consumerDriver) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.Driver.Close()
	})

	return r.closeErr
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
	driver := &consumerDriver{Driver: rawDriver}
	if err = consumer.Prepare(); err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("%s prepare: %w", key, err)
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		defer func() { _ = driver.Close() }()
		if consumeErr := driver.Consumer(consumer.Handle, key); consumeErr != nil {
			color.Errorf("[queue] %s: %v\n", key, consumeErr)
		}
	}()

	return driver, nil
}
