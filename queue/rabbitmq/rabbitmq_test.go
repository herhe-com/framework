package rabbitmq

import (
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/cast"
	"github.com/spf13/viper"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	contractqueue "github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

// queueConfig stubs the configuration of one queue so queueOptions can be
// exercised without booting the application.
type queueConfig struct {
	values map[string]any
}

func (c queueConfig) Env(key string, defaultValue ...any) any { return c.Get(key, defaultValue...) }

func (c queueConfig) Add(string, map[string]any) {}

func (c queueConfig) Set(string, any) {}

func (c queueConfig) Get(key string, defaultValue ...any) any {
	if value, ok := c.values[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (c queueConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := c.values[key]; ok {
		return cast.ToString(value)
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

func (c queueConfig) GetStrings(string, ...[]string) []string { return nil }

func (c queueConfig) GetMaps(string, ...map[string]any) map[string]any { return nil }

func (c queueConfig) GetInt(string, ...int) int { return 0 }

func (c queueConfig) GetInt64(string, ...int64) int64 { return 0 }

func (c queueConfig) GetBool(string, ...bool) bool { return false }

func (c queueConfig) IsSet(key string) bool {
	_, ok := c.values[key]

	return ok
}

func TestConsumerAction(t *testing.T) {
	tests := []struct {
		name     string
		response any
		want     Action
	}{
		{name: "default", want: Ack},
		{name: "ack", response: Ack, want: Ack},
		{name: "discard", response: NackDiscard, want: NackDiscard},
		{name: "requeue", response: NackRequeue, want: NackRequeue},
		{name: "manual unsupported", response: Manual, want: NackRequeue},
		{name: "invalid type", response: "ack", want: NackRequeue},
		{name: "invalid action", response: Action(99), want: NackRequeue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := consumerAction(tt.response); got != tt.want {
				t.Fatalf("consumerAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryAfterMatchesQueueOptions(t *testing.T) {
	options := queueOptions{
		retry: []time.Duration{time.Minute, 5 * time.Minute},
	}

	wait, ok := contractqueue.RetryAfter(options.retry, 0)
	if !ok || wait != time.Minute {
		t.Fatalf("first retry = (%s, %v)", wait, ok)
	}
	wait, ok = contractqueue.RetryAfter(options.retry, 2)
	if ok || wait != 0 {
		t.Fatalf("exhausted retry = (%s, %v)", wait, ok)
	}
	if !contractqueue.HasPositiveRetryDelay(options.retry) {
		t.Fatal("expected positive retry delay")
	}
	if DelayQueuePrefix == "" || DelayExchange == "" {
		t.Fatal("delay names must be set")
	}
}

func TestPublisherKeyDistinguishesConfigurations(t *testing.T) {
	base := queueOptions{
		config: queueconfig.Queue{Key: "email"},
		topic:  "topic",
	}

	keys := map[string]publisherKey{
		"base":                   base.publisherKey(),
		"config":                 queueOptions{config: queueconfig.Queue{Key: "other"}, topic: "topic"}.publisherKey(),
		"topic":                  queueOptions{config: queueconfig.Queue{Key: "email"}, topic: "other"}.publisherKey(),
		"delay default exchange": defaultPublisherKey,
		"delay plugin exchange":  delayPluginPublisherKey,
	}

	seen := make(map[publisherKey]string, len(keys))
	for name, key := range keys {
		if other, exists := seen[key]; exists {
			t.Fatalf("publisher key %v shared by %s and %s", key, other, name)
		}
		seen[key] = name
	}
}

func TestPublisherRejectsAfterClose(t *testing.T) {
	driver := &RabbitMQ{publishers: make(map[publisherKey]*publisher)}

	if err := driver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := driver.publisher(defaultPublisherKey, publishSpec{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("publisher() after Close error = %v, want ErrClosed", err)
	}
}

func TestPublisherRequiresConnection(t *testing.T) {
	driver := &RabbitMQ{publishers: make(map[publisherKey]*publisher)}

	if _, err := driver.publisher(defaultPublisherKey, publishSpec{}); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("publisher() without connection error = %v, want ErrDisconnected", err)
	}

	driver.publishersMu.Lock()
	defer driver.publishersMu.Unlock()
	if len(driver.publishers) != 0 {
		t.Fatalf("publishers after failed creation = %d, want 0", len(driver.publishers))
	}
}

func TestConsumerRequiresConnection(t *testing.T) {
	if _, err := newConsumer(nil, consumerSpec{}); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("newConsumer() without connection error = %v, want ErrDisconnected", err)
	}
}

func TestCloseIsSafeWithoutConnection(t *testing.T) {
	driver := &RabbitMQ{}
	if err := driver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := driver.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestConnectionCurrentStates(t *testing.T) {
	connection := &connection{}

	if _, _, err := connection.current(); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("current() without connection error = %v, want ErrDisconnected", err)
	}

	connection.closed = true
	if _, _, err := connection.current(); !errors.Is(err, ErrClosed) {
		t.Fatalf("current() after close error = %v, want ErrClosed", err)
	}
}

type stubRecoverer struct {
	recovered bool
	stopped   bool
}

func (s *stubRecoverer) recover(*amqp.Connection) error {
	s.recovered = true

	return nil
}

func (s *stubRecoverer) shutdown() {
	s.stopped = true
}

func TestConnectionRegisterRejectsAfterClose(t *testing.T) {
	stub := &stubRecoverer{}
	connection := &connection{closed: true}

	if err := connection.register(stub); !errors.Is(err, ErrClosed) {
		t.Fatalf("register() after close error = %v, want ErrClosed", err)
	}
	if stub.recovered {
		t.Fatal("register() started a recoverer on a closed connection")
	}
}

func TestConnectionBackoffDoublesAndCaps(t *testing.T) {
	connection := &connection{base: time.Second, max: 8 * time.Second}

	cases := []struct {
		current time.Duration
		min     time.Duration
		max     time.Duration
	}{
		{current: time.Second, min: 1500 * time.Millisecond, max: 2500 * time.Millisecond},
		{current: 4 * time.Second, min: 6 * time.Second, max: 10 * time.Second},
		{current: 8 * time.Second, min: 6 * time.Second, max: 10 * time.Second},
	}

	for _, tt := range cases {
		for range 40 {
			got := connection.nextBackoff(tt.current)
			if got < tt.min || got > tt.max {
				t.Fatalf("nextBackoff(%s) = %s, want within [%s, %s]", tt.current, got, tt.min, tt.max)
			}
		}
	}
}

func TestIsClosedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", want: false},
		{name: "closed", err: amqp.ErrClosed, want: true},
		{name: "channel error", err: &amqp.Error{Code: amqp.ChannelError}, want: true},
		{name: "frame error", err: &amqp.Error{Code: amqp.FrameError}, want: true},
		{name: "not found", err: &amqp.Error{Code: amqp.NotFound}, want: false},
		{name: "other", err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClosedError(tt.err); got != tt.want {
				t.Fatalf("isClosedError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestMergeTables(t *testing.T) {
	if got := mergeTables(nil, nil); got != nil {
		t.Fatalf("mergeTables(nil, nil) = %v, want nil", got)
	}

	got := mergeTables(
		amqp.Table{"keep": "base", "override": "base"},
		amqp.Table{"override": "user"},
	)
	want := amqp.Table{"keep": "base", "override": "user"}

	if len(got) != len(want) {
		t.Fatalf("mergeTables() = %v, want %v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("mergeTables()[%s] = %v, want %v", key, got[key], value)
		}
	}
}

func TestTableFromMap(t *testing.T) {
	if got := tableFromMap(nil); got != nil {
		t.Fatalf("tableFromMap(nil) = %v, want nil", got)
	}

	got := tableFromMap(map[string]any{"x-max-length": 10})
	if got["x-max-length"] != 10 {
		t.Fatalf("tableFromMap() = %v, want x-max-length 10", got)
	}
}

func TestQueueOptionsDurabilityDefaultsToTrue(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	facades.Register[contractconfig.Application](queueConfig{values: map[string]any{
		"queue.queues.defaults": map[string]any{
			"topic": "pv_analysis",
			"queue": "pv_analysis_defaults",
		},
		"queue.queues.overridden": map[string]any{
			"topic":            "pv_analysis",
			"queue":            "pv_analysis_overridden",
			"durable":          false,
			"exchange_durable": false,
		},
	}})

	driver := &RabbitMQ{cfg: viper.New()}

	options, err := driver.queueOptions("defaults")
	if err != nil {
		t.Fatalf("queueOptions() error = %v", err)
	}
	if !options.exchangeDurable || !options.durable {
		t.Fatalf(
			"exchange/queue durable = %v/%v, want true/true by default",
			options.exchangeDurable,
			options.durable,
		)
	}

	overridden, err := driver.queueOptions("overridden")
	if err != nil {
		t.Fatalf("queueOptions() error = %v", err)
	}
	if overridden.exchangeDurable || overridden.durable {
		t.Fatalf(
			"exchange/queue durable = %v/%v, want false/false from queue config",
			overridden.exchangeDurable,
			overridden.durable,
		)
	}

	// The connection-level flag stays the fallback of the queue-level one.
	connection := &RabbitMQ{cfg: viper.New()}
	connection.cfg.Set("rabbitmq", map[string]any{"durable": false})

	inherited, err := connection.queueOptions("defaults")
	if err != nil {
		t.Fatalf("queueOptions() error = %v", err)
	}
	if inherited.durable {
		t.Fatal("durable = true, want false from queue.connections.<name>.durable")
	}
}

func TestQueueOptionsHandlerTimeout(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	facades.Register[contractconfig.Application](queueConfig{values: map[string]any{
		"queue.queues.defaults":  map[string]any{"queue": "q_defaults"},
		"queue.queues.custom":    map[string]any{"queue": "q_custom", "handler_timeout": "2m"},
		"queue.queues.unlimited": map[string]any{"queue": "q_unlimited", "handler_timeout": 0},
	}})

	driver := &RabbitMQ{cfg: viper.New()}

	tests := []struct {
		queue string
		want  time.Duration
	}{
		{queue: "defaults", want: contractqueue.DefaultHandlerTimeout},
		{queue: "custom", want: 2 * time.Minute},
		{queue: "unlimited", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.queue, func(t *testing.T) {
			options, err := driver.queueOptions(tt.queue)
			if err != nil {
				t.Fatalf("queueOptions() error = %v", err)
			}
			if options.handlerTimeout != tt.want {
				t.Fatalf("handlerTimeout = %s, want %s", options.handlerTimeout, tt.want)
			}
		})
	}

	// The connection-level value is the fallback of the queue-level one.
	connection := &RabbitMQ{cfg: viper.New()}
	connection.cfg.Set("rabbitmq", map[string]any{"handler_timeout": "90s"})

	options, err := connection.queueOptions("defaults")
	if err != nil {
		t.Fatalf("queueOptions() error = %v", err)
	}
	if options.handlerTimeout != 90*time.Second {
		t.Fatalf("handlerTimeout = %s, want 90s", options.handlerTimeout)
	}
}

func TestHandleHandlerTimeout(t *testing.T) {
	driver := &RabbitMQ{cfg: viper.New()}

	start := time.Now()
	action := driver.handle(
		queueOptions{handlerTimeout: 20 * time.Millisecond},
		func([]byte) (any, error) {
			time.Sleep(500 * time.Millisecond)

			return nil, nil
		},
		amqp.Delivery{Body: []byte("payload")},
	)

	if action != Ack {
		t.Fatalf("handle() action = %v, want Ack", action)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("handle() blocked for %s, want it to return at the timeout", elapsed)
	}
}

func TestBusinessExchangeDefaults(t *testing.T) {
	driver := &RabbitMQ{cfg: viper.New()}

	exchange := driver.businessExchange(queueOptions{topic: "basic", exchangeDurable: true})
	if exchange.kind != amqp.ExchangeDirect {
		t.Fatalf("kind = %q, want %q", exchange.kind, amqp.ExchangeDirect)
	}
	if !exchange.durable {
		t.Fatal("durable = false, want true")
	}
	if !exchange.declare {
		t.Fatal("declare = false, want true")
	}

	// The delayed plugin needs the business exchange to become x-delayed-message
	// with a routing type; durability is unaffected.
	delayed := driver.businessExchange(queueOptions{
		topic:           "basic",
		mode:            delayModePlugin,
		delay:           time.Minute,
		exchangeDurable: true,
	})
	if delayed.kind != "x-delayed-message" {
		t.Fatalf("kind = %q, want x-delayed-message", delayed.kind)
	}
	if delayed.args["x-delayed-type"] != amqp.ExchangeDirect {
		t.Fatalf("x-delayed-type = %v, want direct", delayed.args["x-delayed-type"])
	}
	if !delayed.durable {
		t.Fatal("durable = false, want true")
	}

	// Retries use the shared delay exchange, so the plugin alone must not turn a
	// business exchange without its own delay into x-delayed-message.
	retryOnly := driver.businessExchange(queueOptions{topic: "basic", mode: delayModePlugin})
	if retryOnly.kind != amqp.ExchangeDirect {
		t.Fatalf("kind without a queue delay = %q, want %q", retryOnly.kind, amqp.ExchangeDirect)
	}
}

func TestConsumerExchangesBindsRetryExchangeOnlyWithPlugin(t *testing.T) {
	driver := &RabbitMQ{cfg: viper.New()}
	options := queueOptions{
		topic:  "basic",
		routes: []string{"email", "sms"},
		retry:  []time.Duration{time.Minute},
		mode:   delayModePlugin,
	}

	exchanges := driver.consumerExchanges(options)
	if len(exchanges) != 2 {
		t.Fatalf("exchanges = %d, want 2", len(exchanges))
	}
	if exchanges[0].name != "basic" || len(exchanges[0].bindings) != 2 {
		t.Fatalf("business exchange = %+v", exchanges[0])
	}
	if exchanges[1].name != DelayExchange || len(exchanges[1].bindings) != 2 {
		t.Fatalf("delay exchange = %+v", exchanges[1])
	}

	options.mode = delayModeUndecided
	if exchanges = driver.consumerExchanges(options); len(exchanges) != 1 {
		t.Fatalf("exchanges without plugin = %d, want 1", len(exchanges))
	}
}

func TestQueueArgsAddsTTLDeadLetter(t *testing.T) {
	driver := &RabbitMQ{cfg: viper.New()}

	options := queueOptions{topic: "basic", ttl: time.Minute, queueArgs: amqp.Table{"x-max-length": 5}}
	args := driver.queueArgs(options)

	if args["x-message-ttl"] != int64(60000) {
		t.Fatalf("x-message-ttl = %v, want 60000", args["x-message-ttl"])
	}
	if args["x-dead-letter-exchange"] != "basic" {
		t.Fatalf("x-dead-letter-exchange = %v, want basic", args["x-dead-letter-exchange"])
	}
	if args["x-max-length"] != 5 {
		t.Fatalf("x-max-length = %v, want 5", args["x-max-length"])
	}

	if plain := driver.queueArgs(queueOptions{topic: "basic"}); plain != nil {
		t.Fatalf("queueArgs() without ttl = %v, want nil", plain)
	}
}
