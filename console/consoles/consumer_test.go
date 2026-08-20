package consoles

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	contractqueue "github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/facades"
)

type consumerTestConfig struct {
	values map[string]any
}

func (f consumerTestConfig) Env(key string, defaultValue ...any) any {
	return f.Get(key, defaultValue...)
}

func (f consumerTestConfig) Add(string, map[string]any) {}

func (f consumerTestConfig) Set(string, any) {}

func (f consumerTestConfig) Get(key string, defaultValue ...any) any {
	if value, ok := f.values[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return nil
}

func (f consumerTestConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := f.values[key]; ok {
		return fmt.Sprint(value)
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

func (f consumerTestConfig) GetStrings(string, ...[]string) []string { return nil }

func (f consumerTestConfig) GetMaps(string, ...map[string]any) map[string]any { return nil }

func (f consumerTestConfig) GetInt(string, ...int) int { return 0 }

func (f consumerTestConfig) GetInt64(string, ...int64) int64 { return 0 }

func (f consumerTestConfig) GetBool(string, ...bool) bool { return false }

func (f consumerTestConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

type consumerTestHandler struct {
	key        string
	prepare    error
	handled    chan []byte
	prepareMu  sync.Mutex
	didPrepare bool
}

func (f *consumerTestHandler) Key() string { return f.key }

func (f *consumerTestHandler) Prepare() error {
	f.prepareMu.Lock()
	f.didPrepare = true
	f.prepareMu.Unlock()

	return f.prepare
}

func (f *consumerTestHandler) Handle(data []byte) (any, error) {
	f.handled <- data
	return nil, nil
}

type consumerTestDriver struct {
	consumerStarted chan string
	closed          chan struct{}
	closeOnce       sync.Once
	prepared        func() bool
	orderErr        chan error
}

func (*consumerTestDriver) Producer([]byte, string, ...contractqueue.Headers) error { return nil }

func (f *consumerTestDriver) Consumer(handler contractqueue.Handler, key string) error {
	if f.prepared != nil && !f.prepared() {
		f.orderErr <- errors.New("consumer started before prepare")
		return nil
	}
	f.consumerStarted <- key
	_, _ = handler([]byte("message"))
	return nil
}

func (f *consumerTestDriver) Close() error {
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}

type closeCountingDriver struct {
	consumerTestDriver
	closeCount int
	mu         sync.Mutex
}

func (f *closeCountingDriver) Close() error {
	f.mu.Lock()
	f.closeCount++
	f.mu.Unlock()
	return nil
}

func TestStartConsumerPreparesBeforeConsuming(t *testing.T) {
	setConsumerTestConfig(t, map[string]any{
		"queue.default": "rabbit",
		"queue.queues.search": map[string]any{
			"connection": "events",
		},
	})

	handler := &consumerTestHandler{
		key:     " search ",
		handled: make(chan []byte, 1),
	}
	driver := &consumerTestDriver{
		consumerStarted: make(chan string, 1),
		closed:          make(chan struct{}),
		orderErr:        make(chan error, 1),
		prepared: func() bool {
			handler.prepareMu.Lock()
			defer handler.prepareMu.Unlock()
			return handler.didPrepare
		},
	}
	var connection string
	var wait sync.WaitGroup

	started, err := (&ConsumerProvider{}).startConsumer(handler, &wait, func(name string) (contractqueue.Driver, error) {
		connection = name
		return driver, nil
	})
	if err != nil {
		t.Fatalf("startConsumer() error = %v", err)
	}
	if started == nil {
		t.Fatal("startConsumer() returned nil driver")
	}
	if connection != "events" {
		t.Fatalf("connection = %q, want events", connection)
	}

	select {
	case key := <-driver.consumerStarted:
		if key != "search" {
			t.Fatalf("consumer key = %q, want search", key)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer did not start")
	}

	wait.Wait()
	if err := started.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case err := <-driver.orderErr:
		t.Fatal(err)
	default:
	}
	select {
	case <-driver.closed:
	default:
		t.Fatal("driver was not closed after consumer stopped")
	}
	select {
	case body := <-handler.handled:
		if string(body) != "message" {
			t.Fatalf("handled body = %q", body)
		}
	default:
		t.Fatal("handler was not called")
	}
}

func TestConsumerDriverClosesUnderlyingDriverOnce(t *testing.T) {
	driver := &closeCountingDriver{}
	wrapped := &consumerDriver{Driver: driver}

	if err := wrapped.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := wrapped.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closeCount != 1 {
		t.Fatalf("underlying Close() calls = %d, want 1", driver.closeCount)
	}
}

type blockingConsumerDriver struct {
	started chan struct{}
	closed  chan struct{}
}

func (*blockingConsumerDriver) Producer([]byte, string, ...contractqueue.Headers) error {
	return nil
}

func (f *blockingConsumerDriver) Consumer(contractqueue.Handler, string) error {
	close(f.started)
	<-f.closed
	return nil
}

func (f *blockingConsumerDriver) Close() error {
	select {
	case <-f.closed:
	default:
		close(f.closed)
	}
	return nil
}

func TestStartConsumerCloseUnblocksWait(t *testing.T) {
	setConsumerTestConfig(t, map[string]any{
		"queue.default":       "rabbit",
		"queue.queues.search": map[string]any{},
	})

	driver := &blockingConsumerDriver{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	var wait sync.WaitGroup

	started, err := (&ConsumerProvider{}).startConsumer(
		&consumerTestHandler{key: "search", handled: make(chan []byte, 1)},
		&wait,
		func(string) (contractqueue.Driver, error) { return driver, nil },
	)
	if err != nil {
		t.Fatalf("startConsumer() error = %v", err)
	}

	select {
	case <-driver.started:
	case <-time.After(time.Second):
		t.Fatal("consumer did not start")
	}

	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()

	if err := started.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait group did not finish after Close; consumer.Run was not unblocked")
	}
}

func TestStartConsumerUsesDefaultConnection(t *testing.T) {
	setConsumerTestConfig(t, map[string]any{
		"queue.default":      "rabbit",
		"queue.queues.email": map[string]any{},
	})

	handler := &consumerTestHandler{
		key:     "email",
		handled: make(chan []byte, 1),
	}
	driver := &consumerTestDriver{
		consumerStarted: make(chan string, 1),
		closed:          make(chan struct{}),
	}
	var connection string
	var wait sync.WaitGroup

	_, err := (&ConsumerProvider{}).startConsumer(handler, &wait, func(name string) (contractqueue.Driver, error) {
		connection = name
		return driver, nil
	})
	if err != nil {
		t.Fatalf("startConsumer() error = %v", err)
	}
	wait.Wait()
	if connection != "rabbit" {
		t.Fatalf("connection = %q, want rabbit", connection)
	}
}

func TestStartConsumerStopsWhenPrepareFails(t *testing.T) {
	setConsumerTestConfig(t, map[string]any{
		"queue.default":       "rabbit",
		"queue.queues.search": map[string]any{},
	})

	prepareErr := errors.New("index unavailable")
	handler := &consumerTestHandler{key: "search", prepare: prepareErr}
	driver := &consumerTestDriver{
		consumerStarted: make(chan string, 1),
		closed:          make(chan struct{}),
	}
	var wait sync.WaitGroup

	_, err := (&ConsumerProvider{}).startConsumer(handler, &wait, func(string) (contractqueue.Driver, error) {
		return driver, nil
	})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("startConsumer() error = %v, want %v", err, prepareErr)
	}
	select {
	case <-driver.closed:
	default:
		t.Fatal("driver was not closed after prepare failure")
	}
	select {
	case <-driver.consumerStarted:
		t.Fatal("consumer started after prepare failure")
	default:
	}
}

func setConsumerTestConfig(t *testing.T, values map[string]any) {
	t.Helper()

	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](consumerTestConfig{values: values})
	t.Cleanup(func() { facades.SetContainer(original) })
}
