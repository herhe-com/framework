package rabbitmq

import (
	"errors"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestDelayMilliseconds(t *testing.T) {
	tests := []struct {
		name string
		wait time.Duration
		want int64
	}{
		{name: "zero", wait: 0, want: 0},
		{name: "negative", wait: -time.Second, want: 0},
		{name: "sub-millisecond rounds up", wait: time.Nanosecond, want: 1},
		{name: "partial millisecond rounds up", wait: 1500 * time.Microsecond, want: 2},
		{name: "exact", wait: 30 * time.Minute, want: 1_800_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := delayMilliseconds(tt.wait); got != tt.want {
				t.Fatalf("delayMilliseconds(%s) = %d, want %d", tt.wait, got, tt.want)
			}
		})
	}
}

func TestNewDelayQueue(t *testing.T) {
	queue := newDelayQueue("basic", "email", 90*time.Second)

	if queue.exchange != "basic" {
		t.Fatalf("exchange = %q, want basic", queue.exchange)
	}
	if queue.routingKey != "email" {
		t.Fatalf("routingKey = %q, want email", queue.routingKey)
	}
	if queue.ttl != 90*time.Second {
		t.Fatalf("ttl = %s, want 1m30s", queue.ttl)
	}
	if queue.name != delayQueueName("basic", "email", 90_000) {
		t.Fatalf("name = %q, want the queue name of 90000ms", queue.name)
	}
	// The TTL is derived from the same millisecond value encoded in the name, so
	// a waiting room never mixes messages with different deadlines.
	if !strings.Contains(queue.name, ".90000.") {
		t.Fatalf("name = %q, want it to encode 90000ms", queue.name)
	}
}

func TestDelayQueueNameIsStableAndDistinct(t *testing.T) {
	first := delayQueueName("basic", "email", 60_000)

	if again := delayQueueName("basic", "email", 60_000); again != first {
		t.Fatalf("delayQueueName() = %q then %q, want stable", first, again)
	}

	others := map[string]string{
		"topic":   delayQueueName("events", "email", 60_000),
		"route":   delayQueueName("basic", "sms", 60_000),
		"delay":   delayQueueName("basic", "email", 60_001),
		"partial": delayQueueName("basic", "e", 60_000),
	}

	seen := map[string]string{first: "first"}
	for name, value := range others {
		if other, exists := seen[value]; exists {
			t.Fatalf("delay queue %q shared by %s and %s", value, other, name)
		}
		seen[value] = name
	}
}

func TestDelayQueueNameSanitizesUnsafeCharacters(t *testing.T) {
	name := delayQueueName("basic/events", "user_created *", 1_000)

	if delayQueueName("basic/events", "user_created *", 1_000) != name {
		t.Fatal("sanitized delay queue name is not deterministic")
	}
	// Sanitization is lossy, so mangled names fall back to a hash instead of
	// risking two routing targets sharing a queue with different arguments.
	if strings.Contains(name, "basic") || strings.Contains(name, "user_created") {
		t.Fatalf("delay queue name %q should be hashed, not sanitized", name)
	}
	if strings.ContainsAny(name, " /*") {
		t.Fatalf("delay queue name %q contains unsafe characters", name)
	}
}

func TestDelayQueueNameHashesLongNames(t *testing.T) {
	name := delayQueueName(strings.Repeat("topic.", 40), strings.Repeat("route.", 40), 60_000)

	if len(name) > 255 {
		t.Fatalf("delay queue name length = %d, want <= 255", len(name))
	}
	if !strings.HasPrefix(name, DelayQueuePrefix+".60000.") {
		t.Fatalf("delay queue name = %q, want the prefix and the delay", name)
	}
	if name != delayQueueName(strings.Repeat("topic.", 40), strings.Repeat("route.", 40), 60_000) {
		t.Fatal("hashed delay queue name is not deterministic")
	}
}

func TestDeclarerRejectsWhenConnectionIsClosed(t *testing.T) {
	declarer := &declarer{conn: &connection{closed: true}}

	declarer.mu.Lock()
	_, err := declarer.channelLocked()
	declarer.mu.Unlock()

	if !errors.Is(err, ErrClosed) {
		t.Fatalf("channelLocked() error = %v, want ErrClosed", err)
	}
	if _, err = declarer.ensure("basic", "email", time.Minute); !errors.Is(err, ErrClosed) {
		t.Fatalf("ensure() error = %v, want ErrClosed", err)
	}
	if _, err = declarer.mode(); !errors.Is(err, ErrClosed) {
		t.Fatalf("mode() error = %v, want ErrClosed", err)
	}
}

func TestIsUnknownExchangeType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "missing plugin", err: &amqp.Error{Code: amqp.CommandInvalid}, want: true},
		{name: "not implemented", err: &amqp.Error{Code: amqp.NotImplemented}, want: true},
		{name: "access refused", err: &amqp.Error{Code: amqp.AccessRefused}, want: false},
		{name: "missing plugin 4.3+", err: &amqp.Error{Code: amqp.PreconditionFailed, Reason: "unknown exchange type 'x-delayed-message'"}, want: true},
		{name: "missing plugin uppercase", err: &amqp.Error{Code: amqp.PreconditionFailed, Reason: "PRECONDITION_FAILED - Unknown Exchange Type 'x-delayed-message'"}, want: true},
		{name: "inequivalent arg", err: &amqp.Error{Code: amqp.PreconditionFailed, Reason: "inequivalent arg 'type' for exchange 'pv_analysis'"}, want: false},
		{name: "closed", err: amqp.ErrClosed, want: false},
		{name: "plain error", err: errors.New("dial tcp: timeout"), want: false},
		{name: "nil", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnknownExchangeType(tt.err); got != tt.want {
				t.Fatalf("isUnknownExchangeType(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDeclarerModes(t *testing.T) {
	// A forced mode never contacts the broker.
	for _, forced := range []delayMode{delayModePlugin, delayModeTTL} {
		declarer := &declarer{forced: forced}

		mode, err := declarer.mode()
		if err != nil {
			t.Fatalf("mode() error = %v", err)
		}
		if mode != forced {
			t.Fatalf("mode() = %v, want the forced %v", mode, forced)
		}
	}
}

func TestDeclarerForgetDropsCachedDeclaration(t *testing.T) {
	declarer := &declarer{queues: map[string]struct{}{"delay": {}}}

	declarer.forget("delay")

	declarer.mu.Lock()
	defer declarer.mu.Unlock()
	if _, exists := declarer.queues["delay"]; exists {
		t.Fatal("forget() kept the cached declaration")
	}
}

func TestDeclarerCloseClearsState(t *testing.T) {
	declarer := &declarer{queues: map[string]struct{}{"delay": {}}}

	declarer.close()

	if declarer.queues != nil {
		t.Fatalf("queues after close() = %v, want nil", declarer.queues)
	}
	if declarer.ch != nil {
		t.Fatal("close() kept the declaration channel")
	}
}

func TestParseDelayMode(t *testing.T) {
	tests := []struct {
		value   string
		want    delayMode
		wantErr bool
	}{
		{value: "", want: delayModeUndecided},
		{value: "auto", want: delayModeUndecided},
		{value: " AUTO ", want: delayModeUndecided},
		{value: "plugin", want: delayModePlugin},
		{value: "Plugin", want: delayModePlugin},
		{value: "x-delayed-message", want: delayModePlugin},
		{value: "ttl", want: delayModeTTL},
		{value: "dead-letter", want: delayModeTTL},
		{value: "carrier-pigeon", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseDelayMode(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDelayMode(%q) error = nil, want error", tt.value)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseDelayMode(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("parseDelayMode(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestDelayModeString(t *testing.T) {
	for mode, want := range map[delayMode]string{
		delayModeUndecided: "undecided",
		delayModePlugin:    "plugin (x-delayed-message)",
		delayModeTTL:       "ttl (dead-letter)",
		delayMode(9):       "undecided",
	} {
		if got := mode.String(); got != want {
			t.Fatalf("delayMode(%d).String() = %q, want %q", int(mode), got, want)
		}
	}
}
