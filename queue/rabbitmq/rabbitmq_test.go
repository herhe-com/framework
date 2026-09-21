package rabbitmq

import (
	"errors"
	"testing"
	"time"

	queueclient "github.com/wagslane/go-rabbitmq"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

func TestConsumerAction(t *testing.T) {
	tests := []struct {
		name     string
		response any
		want     queueclient.Action
	}{
		{name: "default", want: queueclient.Ack},
		{name: "ack", response: queueclient.Ack, want: queueclient.Ack},
		{name: "discard", response: queueclient.NackDiscard, want: queueclient.NackDiscard},
		{name: "requeue", response: queueclient.NackRequeue, want: queueclient.NackRequeue},
		{name: "manual unsupported", response: queueclient.Manual, want: queueclient.NackRequeue},
		{name: "invalid type", response: "ack", want: queueclient.NackRequeue},
		{name: "invalid action", response: queueclient.Action(99), want: queueclient.NackRequeue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := consumerAction(tt.response); got != tt.want {
				t.Fatalf("consumerAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackConsumerRejectsAfterClose(t *testing.T) {
	driver := &RabbitMQ{}

	if err := driver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if driver.trackConsumer(&queueclient.Consumer{}) {
		t.Fatal("trackConsumer() = true after Close, want false")
	}

	driver.consumersMu.Lock()
	defer driver.consumersMu.Unlock()
	if len(driver.consumers) != 0 {
		t.Fatalf("consumers after Close = %d, want 0", len(driver.consumers))
	}
}

func TestCloseIsSafeWithoutConsumersOrConnection(t *testing.T) {
	driver := &RabbitMQ{}
	if err := driver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := driver.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
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
	if DelayExchange == "" {
		t.Fatal("DelayExchange must be set")
	}
}

func TestPublisherKeyDistinguishesConfigurations(t *testing.T) {
	base := queueOptions{
		config: queueconfig.Queue{Key: "email"},
		topic:  "topic",
	}

	keys := map[string]publisherKey{
		"base":    base.publisherKey(),
		"config":  queueOptions{config: queueconfig.Queue{Key: "other"}, topic: "topic"}.publisherKey(),
		"topic":   queueOptions{config: queueconfig.Queue{Key: "email"}, topic: "other"}.publisherKey(),
		"delayed": queueOptions{config: queueconfig.Queue{Key: "email"}, topic: "topic", delayed: true}.publisherKey(),
		"ttl":     queueOptions{config: queueconfig.Queue{Key: "email"}, topic: "topic", ttl: time.Second}.publisherKey(),
		"retry":   retryPublisherKey,
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
	driver := &RabbitMQ{}

	if err := driver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := driver.publisher(retryPublisherKey, retryPublisherOptions); !errors.Is(err, ErrClosed) {
		t.Fatalf("publisher() after Close error = %v, want ErrClosed", err)
	}
}

func TestPublisherRequiresConnection(t *testing.T) {
	// Initialize publishers to a non-nil map so that the post-call assertion
	// is meaningful: a nil map has len 0 unconditionally, making the original
	// check a no-op regardless of what publisher() actually does.
	driver := &RabbitMQ{
		publishers: make(map[publisherKey]*queueclient.Publisher),
	}

	if _, err := driver.publisher(retryPublisherKey, retryPublisherOptions); err == nil {
		t.Fatal("publisher() without connection error = nil, want error")
	}

	driver.publishersMu.Lock()
	defer driver.publishersMu.Unlock()
	if len(driver.publishers) != 0 {
		t.Fatalf("publishers after failed creation = %d, want 0", len(driver.publishers))
	}
}
