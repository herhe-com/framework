package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gookit/color"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/cast"
	"github.com/spf13/viper"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

// ErrClosed is returned when publishing or consuming on a closed driver.
var ErrClosed = errors.New("rabbitmq: driver is closed")

const (
	defaultHeartbeat        = 10 * time.Second
	defaultReconnect        = 3 * time.Second
	defaultReconnectRetries = 10
)

type RabbitMQ struct {
	conn *connection

	cfg      *viper.Viper
	name     string
	host     string
	port     int
	username string
	password string
	vhost    string

	publishersMu sync.Mutex
	publishers   map[publisherKey]*publisher

	declarer *declarer
	closed   atomic.Bool
}

type queueOptions struct {
	config queueconfig.Queue
	topic  string
	queue  string
	routes []string
	delay  time.Duration
	ttl    time.Duration
	// mode is the delay implementation probed for this queue. It is filled only
	// when the queue configures a delay or a positive retry wait; otherwise it
	// stays undecided.
	mode            delayMode
	retry           []time.Duration
	concurrency     int
	prefetch        int
	durable         bool
	exchangeDurable bool
	persistent      bool
	exchangeKind    string
	exchangeArgs    amqp.Table
	queueArgs       amqp.Table
	headers         contractqueue.Headers
	errorEnabled    bool
	errorQueue      string
	handlerTimeout  time.Duration
}

// NewRabbitMQ creates a RabbitMQ driver from queue.connections.<name>. The
// connection itself is opened eagerly so configuration errors surface at
// startup; afterwards the client recovers it automatically.
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
	vhost := strings.TrimLeft(cfg.GetString("rabbitmq.vhost"), "/")

	// delay_mode forces a delay implementation; auto probes the broker instead.
	mode, err := parseDelayMode(cfg.GetString("rabbitmq.delay_mode"))
	if err != nil {
		return nil, err
	}

	r := &RabbitMQ{
		cfg:        cfg,
		name:       name,
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		vhost:      vhost,
		publishers: make(map[publisherKey]*publisher),
	}

	conn, err := newConnection(r.url(), r.label(), r.amqpConfig(), r.reconnectInterval())
	if err != nil {
		return nil, err
	}

	r.conn = conn
	r.declarer = &declarer{conn: conn, forced: mode}

	return r, nil
}

func (r *RabbitMQ) url() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s", r.username, url.QueryEscape(r.password), r.host, r.port, r.vhost)
}

// label identifies the broker in logs without leaking credentials.
func (r *RabbitMQ) label() string {
	return fmt.Sprintf("%s:%d/%s", r.host, r.port, r.vhost)
}

func (r *RabbitMQ) reconnectInterval() time.Duration {
	interval := cast.ToDuration(r.cfg.Get("rabbitmq.reconnect"))
	if interval <= 0 {
		return defaultReconnect
	}

	return interval
}

// amqpConfig builds the client configuration.
//
// Recovery is what makes a short disconnect transparent: the client reopens
// channels, re-declares the tracked topology and re-subscribes consumers on the
// very same delivery channels, so worker goroutines keep running. Only when
// those retries are exhausted does the driver rebuild the connection itself
// (see connection.watch).
func (r *RabbitMQ) amqpConfig() amqp.Config {
	heartbeat := cast.ToDuration(r.cfg.Get("rabbitmq.heartbeat"))
	if heartbeat <= 0 {
		heartbeat = defaultHeartbeat
	}

	retries := r.cfg.GetInt("rabbitmq.reconnect_retries")
	if retries <= 0 {
		retries = defaultReconnectRetries
	}

	return amqp.Config{
		Heartbeat: heartbeat,
		Recovery: &amqp.Recovery{
			ReconnectionConfig: &amqp.ReconnectionConfig{
				MaxRetryCount: retries,
				RetryInterval: r.reconnectInterval(),
			},
		},
	}
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
) error {
	if err := r.CheckQueue(options.queue); err != nil {
		return err
	}

	// Delayed publishing never touches the business exchange type: either the
	// plugin exchange holds the message, or a TTL queue dead-letters it back.
	if options.delay > 0 {
		return r.publishDelayed(data, options, routes, headers)
	}

	publisher, err := r.publisher(options.publisherKey(), r.publishSpec(options))
	if err != nil {
		return err
	}

	return publisher.publish(context.Background(), options.topic, routes, false, r.publishing(options, data, headers))
}

// publishSpec describes the publisher channel of one queue: which exchange it
// declares and on which channel it publishes.
func (r *RabbitMQ) publishSpec(options queueOptions) publishSpec {
	exchange := r.businessExchange(options)

	return publishSpec{
		exchange: exchange.name,
		kind:     exchange.kind,
		durable:  exchange.durable,
		declare:  exchange.declare,
		args:     exchange.args,
	}
}

// businessExchange returns the exchange declaration of a queue. The default
// kind is direct and exchanges are declared durable by default; both can be
// overridden per queue or per connection.
func (r *RabbitMQ) businessExchange(options queueOptions) exchangeDecl {
	kind := options.exchangeKind
	if kind == "" {
		kind = r.cfg.GetString("rabbitmq.exchange_kind")
	}
	if kind == "" {
		kind = amqp.ExchangeDirect
	}

	args := options.exchangeArgs
	if len(args) == 0 {
		args = tableFromMap(cast.ToStringMap(r.cfg.Get("rabbitmq.exchange_args")))
	}

	if options.mode == delayModePlugin && options.delay > 0 {
		// The plugin needs its own exchange type, and that type needs to know how
		// to route once the delay elapsed.
		kind = "x-delayed-message"
		args = mergeTables(amqp.Table{"x-delayed-type": amqp.ExchangeDirect}, args)
	}

	return exchangeDecl{
		name:    options.topic,
		kind:    kind,
		durable: options.exchangeDurable,
		declare: true,
		args:    args,
	}
}

// publishing builds the AMQP message of one publish. Delivery mode stays
// transient unless the queue opts into persistent delivery.
func (r *RabbitMQ) publishing(
	options queueOptions,
	data []byte,
	headers contractqueue.Headers,
) amqp.Publishing {
	message := amqp.Publishing{
		Body:         data,
		DeliveryMode: amqp.Transient,
		Headers:      amqp.Table(headers),
	}

	if options.persistent {
		message.DeliveryMode = amqp.Persistent
	}
	if options.ttl > 0 {
		message.Expiration = strconv.FormatInt(options.ttl.Milliseconds(), 10)
	}

	return message
}

// publisher returns the cached publisher of key, creating it on first use. Each
// publisher owns one channel on the shared connection, so the channel count is
// bounded by configuration instead of one channel per message.
func (r *RabbitMQ) publisher(key publisherKey, spec publishSpec) (*publisher, error) {
	r.publishersMu.Lock()
	defer r.publishersMu.Unlock()

	if r.closed.Load() {
		return nil, ErrClosed
	}

	if cached, ok := r.publishers[key]; ok {
		return cached, nil
	}
	if r.conn == nil {
		return nil, ErrDisconnected
	}

	created, err := newPublisher(r.conn, spec)
	if err != nil {
		return nil, err
	}

	// Close() stores closed=true before taking publishersMu, so a non-nil map is
	// guaranteed here; keep the guard for future refactors.
	if r.publishers == nil {
		created.shutdown()
		return nil, ErrClosed
	}

	r.publishers[key] = created

	return created, nil
}

// publisherKey identifies the long-lived publisher of a queue definition. Two
// queues share a publisher only when they declare the same exchange under the
// same configuration key; the empty key is the publisher of the default
// exchange.
type publisherKey struct {
	config string
	topic  string
}

// publisherKey returns the cache key of the queue's publisher.
func (o queueOptions) publisherKey() publisherKey {
	return publisherKey{
		config: o.config.Key,
		topic:  o.topic,
	}
}

// defaultPublisherKey identifies the publisher that writes to the default
// exchange, used to fill the TTL waiting rooms. Queue publishers always carry a
// configuration key, so they cannot collide with it.
var defaultPublisherKey = publisherKey{}

func (r *RabbitMQ) Consumer(handler contractqueue.Handler, key string) error {
	if handler == nil {
		return errors.New("rabbitmq: handler is required")
	}
	if r.closed.Load() {
		return ErrClosed
	}

	options, err := r.queueOptions(key)
	if err != nil {
		return err
	}

	if err = r.CheckQueue(options.queue); err != nil {
		return err
	}

	spec := consumerSpec{
		queue:     options.queue,
		durable:   options.durable,
		queueArgs: r.queueArgs(options),
		exchanges: r.consumerExchanges(options),
		prefetch:  options.prefetch,
		workers:   options.concurrency,
		handler: func(delivery amqp.Delivery) Action {
			return r.handle(options, handler, delivery)
		},
	}

	consumer, err := newConsumer(r.conn, spec)
	if err != nil {
		return err
	}

	// Blocks until the driver closes, like the previous implementation.
	consumer.wait()

	return nil
}

// consumerExchanges returns the exchanges a consuming queue binds itself to.
func (r *RabbitMQ) consumerExchanges(options queueOptions) []exchangeDecl {
	exchange := r.businessExchange(options)
	exchange.bindings = options.routes

	exchanges := []exchangeDecl{exchange}

	// Retries go through the shared plugin exchange when the broker has it, so
	// the queue must be bound to it. TTL waiting rooms dead-letter into the
	// business exchange instead and need no extra binding.
	if options.mode == delayModePlugin && contractqueue.HasPositiveRetryDelay(options.retry) {
		exchanges = append(exchanges, exchangeDecl{
			name:     DelayExchange,
			kind:     "x-delayed-message",
			durable:  true,
			declare:  true,
			args:     amqp.Table{"x-delayed-type": amqp.ExchangeDirect},
			bindings: options.routes,
		})
	}

	return exchanges
}

// queueArgs returns the queue declaration arguments, adding the ttl dead-letter
// pair when the queue configures a message TTL.
func (r *RabbitMQ) queueArgs(options queueOptions) amqp.Table {
	if options.ttl <= 0 {
		return options.queueArgs
	}

	args := amqp.Table{
		"x-message-ttl":          options.ttl.Milliseconds(),
		"x-dead-letter-exchange": options.topic,
	}
	for key, value := range options.queueArgs {
		args[key] = value
	}

	return args
}

// handle runs the queue handler and applies the retry and error-queue policy.
func (r *RabbitMQ) handle(
	options queueOptions,
	handler contractqueue.Handler,
	delivery amqp.Delivery,
) Action {
	headers := contractqueue.Headers(delivery.Headers)
	retried := contractqueue.RetryCount(headers)
	response, consumeErr := contractqueue.CallHandler("rabbitmq", handler, delivery.Body, options.handlerTimeout)

	if consumeErr == nil {
		return consumerAction(response)
	}

	if wait, ok := contractqueue.RetryAfter(options.retry, retried); ok {
		route := delivery.RoutingKey
		if route == "" && len(options.routes) > 0 {
			route = options.routes[0]
		}

		err := r.publishRetry(
			delivery.Body,
			options,
			route,
			contractqueue.WithRetryCount(headers, retried+1),
			wait,
		)
		if err != nil {
			return NackRequeue
		}

		return consumerAction(response)
	}

	if options.errorEnabled {
		data := contractqueue.BasicError{
			Exchange: options.topic,
			Queue:    options.queue,
			Route:    delivery.RoutingKey,
			Retry:    retried,
			Message:  string(delivery.Body),
			Error:    consumeErr.Error(),
		}

		body, err := json.Marshal(data)
		if err != nil {
			color.Errorf("[queue.rabbitmq] encode error message: %v\n", err)

			return NackRequeue
		}

		errorOptions := queueOptions{
			topic:  options.errorQueue,
			queue:  options.errorQueue,
			routes: []string{options.errorQueue},
		}
		if err = r.publish(body, errorOptions, errorOptions.routes, nil); err != nil {
			return NackRequeue
		}
	}

	return consumerAction(response)
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

	// 'delayed' is a legacy switch that forced an x-delayed-message exchange.
	// The exchange type is now derived from the broker capability instead, so the
	// flag is ignored.

	queueName := config.String("queue", key)
	routes := config.Strings("routes")
	if len(routes) == 0 {
		routes = []string{config.String("route", queueName)}
	}
	concurrency := config.Int("concurrency", 10)
	if concurrency <= 0 {
		concurrency = 10
	}
	prefetch := config.Int("prefetch", concurrency)
	if prefetch <= 0 {
		prefetch = concurrency
	}
	retry, err := config.Retry()
	if err != nil {
		return queueOptions{}, err
	}
	errorQueue := config.String("error", r.cfg.GetString("rabbitmq.error"))
	if errorQueue == "" {
		errorQueue = "basic_error"
	}

	// Delays use the plugin exchange when the broker has it (RabbitMQ 4.2 and
	// earlier with rabbitmq_delayed_message_exchange) and TTL waiting queues
	// otherwise. Only queues that actually delay probe the broker, so a queue
	// without delay or retry pays nothing for the capability check.
	mode := delayModeUndecided
	if delay > 0 || contractqueue.HasPositiveRetryDelay(retry) {
		mode, err = r.declarer.mode()
		if err != nil {
			return queueOptions{}, err
		}
	}

	// Exchanges and queues are durable by default: RabbitMQ refuses to change
	// either flag afterwards, so a non-durable redeclaration of an existing
	// durable exchange or queue fails with 406 PRECONDITION_FAILED.
	exchangeDurable := config.Bool("exchange_durable", true)
	durable := true
	if r.cfg.IsSet("rabbitmq.durable") {
		durable = r.cfg.GetBool("rabbitmq.durable")
	}

	// A handler that never returns would hold a consumer worker and its message
	// forever; on timeout the message follows the retry ladder instead.
	handlerTimeout := contractqueue.DefaultHandlerTimeout
	if r.cfg.IsSet("rabbitmq.handler_timeout") {
		handlerTimeout = r.cfg.GetDuration("rabbitmq.handler_timeout")
	}
	handlerTimeout, err = config.Duration("handler_timeout", handlerTimeout)
	if err != nil {
		return queueOptions{}, err
	}

	return queueOptions{
		config:          config,
		topic:           config.String("topic", queueName),
		queue:           queueName,
		routes:          routes,
		delay:           delay,
		ttl:             ttl,
		mode:            mode,
		retry:           retry,
		concurrency:     concurrency,
		prefetch:        prefetch,
		durable:         config.Bool("durable", durable),
		exchangeDurable: exchangeDurable,
		persistent:      config.Bool("persistent", r.cfg.GetBool("rabbitmq.persistent")),
		exchangeKind:    config.String("exchange_kind", r.cfg.GetString("rabbitmq.exchange_kind")),
		exchangeArgs:    tableFromMap(cast.ToStringMap(config.Value("exchange_args"))),
		queueArgs:       tableFromMap(cast.ToStringMap(config.Value("queue_args"))),
		headers:         config.Headers(),
		errorEnabled:    config.Bool("error_enable", true),
		errorQueue:      errorQueue,
		handlerTimeout:  handlerTimeout,
	}, nil
}

func consumerAction(response any) Action {
	if response == nil {
		return Ack
	}

	action, ok := response.(Action)
	if !ok {
		color.Errorf("[queue.rabbitmq] invalid consumer response %T; requeueing message\n", response)

		return NackRequeue
	}

	switch action {
	case Ack, NackDiscard, NackRequeue:
		return action
	default:
		color.Errorf("[queue.rabbitmq] invalid consumer action %d; requeueing message\n", action)

		return NackRequeue
	}
}

func (r *RabbitMQ) Close() error {
	if r.closed.Swap(true) {
		return nil
	}

	var err error

	// The connection stops every consumer (waiting for in-flight handlers) and
	// then closes the socket.
	if r.conn != nil {
		err = r.conn.Close()
	}

	r.publishersMu.Lock()
	publishers := r.publishers
	r.publishers = nil
	r.publishersMu.Unlock()

	for _, publisher := range publishers {
		publisher.shutdown()
	}

	if r.declarer != nil {
		r.declarer.close()
	}

	return err
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
