package rabbitmq

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
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
		rabbitmq.WithConnectionOptionsBaseReconnectInterval(3 * time.Second),
	}

	return rabbitmq.NewConn(r.url(), options...)
}

func (r *RabbitMQ) Producer(data []byte, options contractqueue.ProducerOptions) (err error) {

	if err = r.CheckQueue(options.Queue); err != nil {
		return err
	}

	var publisher *rabbitmq.Publisher

	publisherOptions := r.PublisherOptions(options.Queue)

	if options.Delay > 0 {
		publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, publisherOptions...)
	} else if options.TTL > 0 {
		publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, publisherOptions...)
	}

	publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
		rabbitmq.WithPublisherOptionsExchangeName(options.Topic),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
	}, publisherOptions...)

	if publisher, err = rabbitmq.NewPublisher(r.conn, publisherOptions...); err != nil {
		return err
	}

	opts := r.PublishOptions(options.Queue)
	opts = append(opts, rabbitmq.WithPublishOptionsExchange(options.Topic))
	if options.TTL > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsExpiration(strconv.FormatInt(options.TTL.Milliseconds(), 10)))
	}

	header := rabbitmq.Table{}

	for key, value := range options.Headers {
		header[key] = value
	}
	if options.Delay > 0 {
		header[contractqueue.DelayHeader] = options.Delay.Milliseconds()
	}

	if len(header) > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsHeaders(header))
	}

	return publisher.Publish(data, options.Routes, opts...)
}

func (r *RabbitMQ) Consumer(handler contractqueue.Handler, options contractqueue.ConsumerOptions) (err error) {

	if err = r.CheckQueue(options.Queue); err != nil {
		return err
	}

	consumerOptions := r.ConsumerOptions(options.Queue)

	if options.Delayed {

		consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsExchangeArgs(rabbitmq.Table{
				"x-delayed-type": "direct",
			}),
			rabbitmq.WithConsumerOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, consumerOptions...)
	}
	if options.TTL > 0 {

		consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
				"x-message-ttl":          options.TTL.Milliseconds(),
				"x-dead-letter-exchange": options.Topic,
			}),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, consumerOptions...)
	}

	consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsExchangeName(options.Topic),
		rabbitmq.WithConsumerOptionsRoutingKey(options.Route),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsConcurrency(10),
	}, consumerOptions...)

	consumer, err := rabbitmq.NewConsumer(r.conn, options.Queue, consumerOptions...)

	if err != nil {
		return err
	}

	err = consumer.Run(func(d rabbitmq.Delivery) (action rabbitmq.Action) {
		headers := contractqueue.Headers(d.Headers)
		retried := contractqueue.RetryCount(headers)

		if consumeErr := handler(d.Body); consumeErr != nil {

			if options.Retry > 0 && retried < options.Retry {
				if retryErr := r.Producer(d.Body, contractqueue.ProducerOptions{
					Topic:   options.Topic,
					Queue:   options.Queue,
					Routes:  []string{options.Route},
					Delay:   contractqueue.RetryDelay(headers),
					TTL:     options.TTL,
					Headers: contractqueue.WithRetryCount(headers, retried+1),
				}); retryErr != nil {
					return rabbitmq.NackRequeue
				}

				return rabbitmq.Ack
			}

			q := r.cfg.GetString("rabbitmq.error")

			if q == "" {
				q = "basic_error"
			}

			data := contractqueue.BasicError{
				Exchange: options.Topic,
				Queue:    options.Queue,
				Route:    options.Route,
				Retry:    retried,
				Message:  string(d.Body),
				Error:    consumeErr.Error(),
			}

			body, _ := json.Marshal(data)

			if publishErr := r.Producer(body, contractqueue.ProducerOptions{
				Topic:  q,
				Queue:  q,
				Routes: []string{q},
			}); publishErr != nil {
				return rabbitmq.NackRequeue
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
