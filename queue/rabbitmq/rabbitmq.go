package rabbitmq

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/spf13/viper"
	"github.com/wagslane/go-rabbitmq"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

// DelayExchange is the shared x-delayed-message exchange used only for
// consumer retry requeues. Business exchanges stay unchanged.
const DelayExchange = "_framework.queue.delay"

type RabbitMQ struct {
	conn        *rabbitmq.Conn
	cfg         *viper.Viper
	name        string
	host        string
	port        int
	username    string
	password    string
	vhost       string
	consumersMu sync.Mutex
	consumers   []*rabbitmq.Consumer
	closed      bool
}

type queueOptions struct {
	config       queueconfig.Queue
	topic        string
	queue        string
	routes       []string
	delay        time.Duration
	ttl          time.Duration
	delayed      bool
	retry        []time.Duration
	concurrency  int
	headers      contractqueue.Headers
	errorEnabled bool
	errorQueue   string
}

func NewRabbitMQ(configs map[string]any, names ...string) (queue *RabbitMQ, err error) {
	name := ""
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		name = strings.TrimSpace(names[0])
	}

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
		name:     name,
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

func (r *RabbitMQ) Producer(data []byte, key string, headers ...contractqueue.Headers) error {
	options, err := r.queueOptions(key)
	if err != nil {
		return err
	}

	return r.publish(data, options, options.routes, contractqueue.MergeHeaders(options.headers, contractqueue.MergeHeaders(headers...)))
}

func (r *RabbitMQ) publish(
	data []byte,
	options queueOptions,
	routes []string,
	headers contractqueue.Headers,
) (err error) {
	if err = r.CheckQueue(options.queue); err != nil {
		return err
	}

	var publisher *rabbitmq.Publisher

	publisherOptions := r.PublisherOptions(options)

	if options.delayed {
		publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, publisherOptions...)
	} else if options.ttl > 0 {
		publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
			rabbitmq.WithPublisherOptionsExchangeDurable,
		}, publisherOptions...)
	}

	publisherOptions = append([]func(publisherOptions *rabbitmq.PublisherOptions){
		rabbitmq.WithPublisherOptionsExchangeName(options.topic),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
	}, publisherOptions...)

	if publisher, err = rabbitmq.NewPublisher(r.conn, publisherOptions...); err != nil {
		return err
	}

	opts := r.PublishOptions(options)
	opts = append(opts, rabbitmq.WithPublishOptionsExchange(options.topic))
	if options.ttl > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsExpiration(strconv.FormatInt(options.ttl.Milliseconds(), 10)))
	}

	header := rabbitmq.Table{}

	for key, value := range headers {
		header[key] = value
	}
	if options.delay > 0 {
		header[contractqueue.DelayHeader] = options.delay.Milliseconds()
	}

	if len(header) > 0 {
		opts = append(opts, rabbitmq.WithPublishOptionsHeaders(header))
	}

	return publisher.Publish(data, routes, opts...)
}

// publishRetry requeues a failed message. Positive waits go through the shared
// delay exchange so business exchanges stay on their original type.
func (r *RabbitMQ) publishRetry(
	data []byte,
	options queueOptions,
	route string,
	headers contractqueue.Headers,
	wait time.Duration,
) error {
	if wait <= 0 {
		return r.publish(data, options, []string{route}, headers)
	}

	if err := r.CheckQueue(options.queue); err != nil {
		return err
	}

	publisher, err := rabbitmq.NewPublisher(r.conn,
		rabbitmq.WithPublisherOptionsExchangeName(DelayExchange),
		rabbitmq.WithPublisherOptionsExchangeKind("x-delayed-message"),
		rabbitmq.WithPublisherOptionsExchangeDurable,
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsExchangeArgs(rabbitmq.Table{
			"x-delayed-type": "direct",
		}),
	)
	if err != nil {
		return err
	}

	header := rabbitmq.Table{}
	for key, value := range headers {
		header[key] = value
	}
	header[contractqueue.DelayHeader] = wait.Milliseconds()

	return publisher.Publish(
		data,
		[]string{route},
		rabbitmq.WithPublishOptionsExchange(DelayExchange),
		rabbitmq.WithPublishOptionsHeaders(header),
	)
}

func (r *RabbitMQ) Consumer(handler contractqueue.Handler, key string) (err error) {
	if handler == nil {
		return errors.New("rabbitmq: handler is required")
	}

	options, err := r.queueOptions(key)
	if err != nil {
		return err
	}

	if err = r.CheckQueue(options.queue); err != nil {
		return err
	}

	consumerOptions := r.ConsumerOptions(options)

	if options.delayed {

		consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsExchangeArgs(rabbitmq.Table{
				"x-delayed-type": "direct",
			}),
			rabbitmq.WithConsumerOptionsExchangeKind("x-delayed-message"),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, consumerOptions...)
	}
	if options.ttl > 0 {

		consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
			rabbitmq.WithConsumerOptionsQueueArgs(rabbitmq.Table{
				"x-message-ttl":          options.ttl.Milliseconds(),
				"x-dead-letter-exchange": options.topic,
			}),
			rabbitmq.WithConsumerOptionsExchangeDurable,
		}, consumerOptions...)
	}

	consumerOptions = append([]func(*rabbitmq.ConsumerOptions){
		rabbitmq.WithConsumerOptionsExchangeName(options.topic),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsConcurrency(options.concurrency),
	}, consumerOptions...)
	for _, route := range options.routes {
		consumerOptions = append(consumerOptions, rabbitmq.WithConsumerOptionsRoutingKey(route))
	}
	if contractqueue.HasPositiveRetryDelay(options.retry) {
		bindings := make([]rabbitmq.Binding, 0, len(options.routes))
		for _, route := range options.routes {
			bindings = append(bindings, rabbitmq.Binding{
				RoutingKey: route,
				BindingOptions: rabbitmq.BindingOptions{
					Declare: true,
				},
			})
		}
		consumerOptions = append(consumerOptions, rabbitmq.WithConsumerOptionsExchangeOptions(rabbitmq.ExchangeOptions{
			Name:     DelayExchange,
			Kind:     "x-delayed-message",
			Durable:  true,
			Declare:  true,
			Args:     rabbitmq.Table{"x-delayed-type": "direct"},
			Bindings: bindings,
		}))
	}

	consumer, err := rabbitmq.NewConsumer(r.conn, options.queue, consumerOptions...)
	if err != nil {
		return err
	}
	if !r.trackConsumer(consumer) {
		consumer.Close()
		return errors.New("rabbitmq: driver is closed")
	}

	// Run blocks until consumer.Close() unblocks reconnectErrCh.
	// Closing only the connection leaves Run hanging forever.
	return consumer.Run(func(d rabbitmq.Delivery) (action rabbitmq.Action) {
		headers := contractqueue.Headers(d.Headers)
		retried := contractqueue.RetryCount(headers)
		response, consumeErr := handler(d.Body)

		if consumeErr != nil {
			if wait, ok := contractqueue.RetryAfter(options.retry, retried); ok {
				route := d.RoutingKey
				if route == "" && len(options.routes) > 0 {
					route = options.routes[0]
				}
				if retryErr := r.publishRetry(
					d.Body,
					options,
					route,
					contractqueue.WithRetryCount(headers, retried+1),
					wait,
				); retryErr != nil {
					return rabbitmq.NackRequeue
				}

				return consumerAction(response)
			}

			if options.errorEnabled {
				data := contractqueue.BasicError{
					Exchange: options.topic,
					Queue:    options.queue,
					Route:    d.RoutingKey,
					Retry:    retried,
					Message:  string(d.Body),
					Error:    consumeErr.Error(),
				}

				body, _ := json.Marshal(data)

				errorOptions := queueOptions{
					topic:  options.errorQueue,
					queue:  options.errorQueue,
					routes: []string{options.errorQueue},
				}
				if publishErr := r.publish(body, errorOptions, errorOptions.routes, nil); publishErr != nil {
					return rabbitmq.NackRequeue
				}
			}

		}

		return consumerAction(response)
	})
}

func (r *RabbitMQ) trackConsumer(consumer *rabbitmq.Consumer) bool {
	r.consumersMu.Lock()
	defer r.consumersMu.Unlock()

	if r.closed {
		return false
	}

	r.consumers = append(r.consumers, consumer)
	return true
}

func (r *RabbitMQ) queueOptions(key string) (queueOptions, error) {
	config, err := queueconfig.Load(key)
	if err != nil {
		return queueOptions{}, err
	}
	if err = config.EnsureEnabled(); err != nil {
		return queueOptions{}, err
	}
	if r.name != "" && config.Connection != r.name {
		return queueOptions{}, fmt.Errorf(
			"queue %s uses connection %s, not %s",
			key,
			config.Connection,
			r.name,
		)
	}

	delay, err := config.Duration("delay", 0)
	if err != nil {
		return queueOptions{}, err
	}
	ttl, err := config.Duration("ttl", 0)
	if err != nil {
		return queueOptions{}, err
	}

	// 'delayed' and 'ttl' use incompatible exchange configurations: delayed
	// queues require x-delayed-message, while TTL queues declare a plain
	// durable exchange with x-message-ttl queue args. Enabling both at once
	// leads to undefined behaviour at the broker level.
	delayed := delay > 0 || config.Bool("delayed", false)
	if delayed && ttl > 0 {
		return queueOptions{}, fmt.Errorf("queue %s: 'delayed' and 'ttl' are mutually exclusive", key)
	}

	queueName := config.String("queue", key)
	routes := config.Strings("routes")
	if len(routes) == 0 {
		routes = []string{config.String("route", queueName)}
	}
	concurrency := config.Int("concurrency", 10)
	if concurrency <= 0 {
		concurrency = 10
	}
	retry, err := config.Retry()
	if err != nil {
		return queueOptions{}, err
	}
	errorQueue := config.String("error", r.cfg.GetString("rabbitmq.error"))
	if errorQueue == "" {
		errorQueue = "basic_error"
	}

	return queueOptions{
		config:       config,
		topic:        config.String("topic", queueName),
		queue:        queueName,
		routes:       routes,
		delay:        delay,
		ttl:          ttl,
		delayed:      delayed,
		retry:        retry,
		concurrency:  concurrency,
		headers:      config.Headers(),
		errorEnabled: config.Bool("error_enable", true),
		errorQueue:   errorQueue,
	}, nil
}

func consumerAction(response any) rabbitmq.Action {
	if response == nil {
		return rabbitmq.Ack
	}

	action, ok := response.(rabbitmq.Action)
	if !ok {
		color.Errorf("[queue.rabbitmq] invalid consumer response %T; requeueing message", response)
		return rabbitmq.NackRequeue
	}

	switch action {
	case rabbitmq.Ack, rabbitmq.NackDiscard, rabbitmq.NackRequeue:
		return action
	default:
		color.Errorf("[queue.rabbitmq] invalid consumer action %d; requeueing message", action)
		return rabbitmq.NackRequeue
	}
}

func (r *RabbitMQ) Close() error {
	r.consumersMu.Lock()
	r.closed = true
	consumers := r.consumers
	r.consumers = nil
	r.consumersMu.Unlock()

	// Close consumers first so consumer.Run() can return; go-rabbitmq requires
	// this before conn.Close(), otherwise graceful connection shutdown leaves
	// Run blocked on reconnectErrCh forever.
	for _, consumer := range consumers {
		consumer.Close()
	}

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

func (r *RabbitMQ) PublisherOptions(queue queueOptions) []func(*rabbitmq.PublisherOptions) {

	var opts []func(*rabbitmq.PublisherOptions)
	var options []func(*rabbitmq.PublisherOptions)

	options, _ = r.cfg.Get("rabbitmq.default.publisher_options").([]func(*rabbitmq.PublisherOptions))
	opts, _ = queue.config.Value("publisher_options").([]func(*rabbitmq.PublisherOptions))

	return append(options, opts...)
}

func (r *RabbitMQ) PublishOptions(queue queueOptions) []func(*rabbitmq.PublishOptions) {

	var options []func(*rabbitmq.PublishOptions)
	var opts []func(*rabbitmq.PublishOptions)

	options, _ = r.cfg.Get("rabbitmq.default.publish_options").([]func(*rabbitmq.PublishOptions))
	opts, _ = queue.config.Value("publish_options").([]func(*rabbitmq.PublishOptions))

	return append(options, opts...)
}

func (r *RabbitMQ) ConsumerOptions(queue queueOptions) []func(options *rabbitmq.ConsumerOptions) {

	var options []func(*rabbitmq.ConsumerOptions)
	var opts []func(*rabbitmq.ConsumerOptions)

	options, _ = r.cfg.Get("rabbitmq.default.consumer_options").([]func(*rabbitmq.ConsumerOptions))
	opts, _ = queue.config.Value("consumer_options").([]func(*rabbitmq.ConsumerOptions))

	return append(options, opts...)
}
