package rabbitmq

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	constnats "github.com/herhe-com/framework/contracts/queue"
	"github.com/spf13/viper"
	"github.com/wagslane/go-rabbitmq"
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
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s", r.username, url.QueryEscape(r.password), r.host, r.port, r.vhost)
}

func (r *RabbitMQ) Conn() (*rabbitmq.Conn, error) {

	options := []func(options *rabbitmq.ConnectionOptions){
		rabbitmq.WithConnectionOptionsReconnectInterval(3 * time.Second),
	}

	return rabbitmq.NewConn(r.url(), options...)
}

func (r *RabbitMQ) Producer(data []byte, exchange, queue string, routes []string, delay, ttl int64, headers ...rabbitmq.Table) (err error) {

	if err = r.CheckQueue(queue); err != nil {
		return err
	}

	var publisher *rabbitmq.Publisher

	options := r.PublisherOptions(queue)

	if delay > 0 {
		options = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, options...)
	} else if ttl > 0 {
		options = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, options...)
	}

	options = append([]func(publisherOptions *rabbitmq.PublisherOptions){
		rabbitmq.WithPublisherOptionsExchangeName(exchange),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
	}, options...)

	if publisher, err = rabbitmq.NewPublisher(r.conn, options...); err != nil {
		return err
	}

	opts := r.PublishOptions(queue)
	opts = append(opts, rabbitmq.WithPublishOptionsExchange(exchange))

	header := rabbitmq.Table{}

	if delay > 0 {
		header["x-delay"] = delay * 1000
	} else if ttl > 0 {
		header["x-message-ttl"] = ttl * 1000
		header["x-dead-letter-exchange"] = exchange
	}

	if len(headers) > 0 {
		for _, value := range headers {
			for k, v := range value {
				header[k] = v
			}
		}
	}

	if len(header) > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsHeaders(header))
	}

	return publisher.Publish(data, routes, opts...)
}

func (r *RabbitMQ) Consumer(handler func(data []byte) error, exchange, queue, route string, delay bool, ttl int64, retry int) (err error) {

	if err = r.CheckQueue(queue); err != nil {
		return err
	}

	options := r.ConsumerOptions(queue)

	if delay {

		options = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsExchangeArgs(rabbitmq.Table{
				"x-delayed-type": "direct",
			}),
			rabbitmq.WithConsumerOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, options...)
	} else if ttl > 0 {

		options = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
				"x-message-ttl":          ttl * 1000,
				"x-dead-letter-exchange": exchange,
			}),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, options...)
	}

	options = append([]func(*rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsExchangeName(exchange),
		rabbitmq.WithConsumerOptionsRoutingKey(route),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsConcurrency(10),
	}, options...)

	consumer, err := rabbitmq.NewConsumer(r.conn, queue, options...)

	if err != nil {
		return err
	}

	err = consumer.Run(func(d rabbitmq.Delivery) (action rabbitmq.Action) {

		retried := 0

		if retry > 0 {
			retried, _ = d.Headers["x-retry"].(int)
		}

		var delayTTTL int64 = 0

		if delay {
			delayTTTL, _ = d.Headers["x-delay"].(int64)
		}

		if err = handler(d.Body); err != nil {

			if retry > 0 && retried < retry {
				d.Headers["x-retry"] = retried + 1

				if err = r.Producer(d.Body, exchange, queue, []string{route}, delayTTTL, ttl); err != nil {
					return rabbitmq.NackDiscard
				}

				return rabbitmq.Ack
			}

			q := r.cfg.GetString("rabbitmq.error")

			if q == "" {
				q = "basic_error"
			}

			data := constnats.BasicError{
				Exchange: exchange,
				Queue:    queue,
				Route:    route,
				Retry:    retried,
				Message:  string(d.Body),
				Error:    err.Error(),
			}

			body, _ := json.Marshal(data)

			if err = r.Producer(body, q, q, []string{q}, 0, 0); err != nil {
				return rabbitmq.NackDiscard
			}
		}

		return rabbitmq.Ack
	})

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
