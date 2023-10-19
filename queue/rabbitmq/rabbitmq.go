package rabbitmq

import (
	"errors"
	"fmt"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"github.com/wagslane/go-rabbitmq"
	"strings"
	"time"
)

type RabbitMQ struct {
	conn     *rabbitmq.Conn
	cfg      *viper.Viper
	host     string
	port     int
	username string
	password string
	vhost    string
}

func NewRabbitMQ(configs map[string]any) (queue *RabbitMQ, err error) {

	cfg := viper.New()

	cfg.Set("rabbitmq", configs)

	host := cfg.GetString("rabbitmq.host")
	port := cfg.GetInt("rabbitmq.port")
	username := cfg.GetString("rabbitmq.username")
	password := cfg.GetString("rabbitmq.password")
	vhost := cfg.GetString("rabbitmq.vhost")

	vhost = strings.TrimLeft(vhost, "/")

	r := &RabbitMQ{
		cfg:      cfg,
		host:     host,
		port:     port,
		username: username,
		password: password,
		vhost:    vhost,
	}

	var conn *rabbitmq.Conn

	if conn, err = r.Conn(); err != nil {
		return nil, err
	}

	r.conn = conn

	return r, nil
}

func (r *RabbitMQ) url() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s", r.username, r.password, r.host, r.port, r.vhost)
}

func (r *RabbitMQ) Conn() (*rabbitmq.Conn, error) {

	options := []func(options *rabbitmq.ConnectionOptions){
		rabbitmq.WithConnectionOptionsReconnectInterval(3 * time.Second),
	}

	return rabbitmq.NewConn(r.url(), options...)
}

func (r *RabbitMQ) Producer(data []byte, queue string, routes []string, delays ...time.Duration) (err error) {

	if err = r.CheckQueue(queue); err != nil {
		return err
	}

	var delay time.Duration = 0

	delays = lo.Filter(delays, func(item time.Duration, index int) bool {
		return item > 0
	})

	if len(delays) > 0 {
		delay = delays[0]
	}

	var publisher *rabbitmq.Publisher

	options := r.PublisherOptions(queue)

	options = append([]func(publisherOptions *rabbitmq.PublisherOptions){
		rabbitmq.WithPublisherOptionsExchangeName(queue),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
	}, options...)

	if publisher, err = rabbitmq.NewPublisher(r.conn, options...); err != nil {
		return err
	}

	opts := r.PublishOptions(queue)
	opts = append(opts, rabbitmq.WithPublishOptionsExchange(queue))

	if delay > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsHeaders(rabbitmq.Table{"x-delay": delay * time.Minute}))
	}

	return publisher.Publish(data, routes, opts...)
}

func (r *RabbitMQ) Consumer(handler func(data []byte), queue string) (err error) {

	if err = r.CheckQueue(queue); err != nil {
		return err
	}

	options := r.ConsumerOptions(queue)

	options = append([]func(consumerOptions *rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsRoutingKey(queue),
		rabbitmq.WithConsumerOptionsExchangeName(queue),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsConcurrency(10),
		rabbitmq.WithConsumerOptionsConcurrency(10),
	}, options...)

	_, err = rabbitmq.NewConsumer(r.conn, func(d rabbitmq.Delivery) (action rabbitmq.Action) {
		handler(d.Body)
		return rabbitmq.Ack
	}, queue, options...)

	if err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) Close() error {

	if r.conn == nil {
		return nil
	}

	return r.conn.Close()
}

func (r *RabbitMQ) CheckQueue(queue string) error {

	if queue == "default" {
		return errors.New("exchange can't be 'default'")
	}

	if strings.Contains(queue, ".") {
		return errors.New("exchange cannot contain '.'")
	}

	return nil
}

func (r *RabbitMQ) PublisherOptions(queue string) []func(*rabbitmq.PublisherOptions) {

	var opts []func(*rabbitmq.PublisherOptions)
	var options []func(*rabbitmq.PublisherOptions)

	options, _ = r.cfg.Get("rabbitmq.default.publisher_options").([]func(*rabbitmq.PublisherOptions))
	opts, _ = r.cfg.Get(fmt.Sprintf("rabbitmq.%s.publish_options", queue)).([]func(*rabbitmq.PublisherOptions))

	return append(options, opts...)
}

func (r *RabbitMQ) PublishOptions(queue string) []func(*rabbitmq.PublishOptions) {

	var options []func(*rabbitmq.PublishOptions)
	var opts []func(*rabbitmq.PublishOptions)

	options, _ = r.cfg.Get("rabbitmq.default.publish_options").([]func(*rabbitmq.PublishOptions))
	opts, _ = r.cfg.Get(fmt.Sprintf("rabbitmq.%s.publish_options", queue)).([]func(*rabbitmq.PublishOptions))

	return append(options, opts...)
}

func (r *RabbitMQ) ConsumerOptions(queue string) []func(options *rabbitmq.ConsumerOptions) {

	var options []func(*rabbitmq.ConsumerOptions)
	var opts []func(*rabbitmq.ConsumerOptions)

	options, _ = r.cfg.Get("rabbitmq.default.consumer_options").([]func(*rabbitmq.ConsumerOptions))
	opts, _ = r.cfg.Get(fmt.Sprintf("rabbitmq.%s.consumer_options", queue)).([]func(*rabbitmq.ConsumerOptions))

	return append(options, opts...)
}
