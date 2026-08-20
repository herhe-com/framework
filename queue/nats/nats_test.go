package nats

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
)

type fakeAcknowledgement struct {
	action Action
	err    error
}

func (f *fakeAcknowledgement) Ack() error {
	f.action = Ack
	return f.err
}

func (f *fakeAcknowledgement) Nak() error {
	f.action = Nak
	return f.err
}

func (f *fakeAcknowledgement) Term() error {
	f.action = Term
	return f.err
}

func TestPublishSubjects(t *testing.T) {
	tests := []struct {
		name   string
		topic  string
		queue  string
		routes []string
		want   []string
	}{
		{
			name:   "topic and routes",
			topic:  "events",
			routes: []string{"created", ".updated."},
			want:   []string{"events.created", "events.updated"},
		},
		{
			name:   "full route",
			routes: []string{"events.created"},
			want:   []string{"events.created"},
		},
		{
			name:  "topic fallback",
			topic: "events",
			queue: "workers",
			want:  []string{"events"},
		},
		{
			name:  "queue fallback",
			queue: "workers",
			want:  []string{"workers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publishSubjects(tt.topic, tt.queue, tt.routes); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("publishSubjects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetHeaders(t *testing.T) {
	message := natsgo.NewMsg("events.created")
	setHeaders(message, contractqueue.Headers{
		"attempt": 2,
		"tags":    []string{"email", "urgent"},
	})

	if got := message.Header.Get("attempt"); got != "2" {
		t.Fatalf("attempt header = %q, want %q", got, "2")
	}
	if got := message.Header.Values("tags"); !reflect.DeepEqual(got, []string{"email", "urgent"}) {
		t.Fatalf("tags header = %v, want %v", got, []string{"email", "urgent"})
	}
}

func TestHeadersFromNATS(t *testing.T) {
	source := natsgo.Header{
		"trace-id":      []string{"request-1"},
		"tags":          []string{"email", "urgent"},
		"X-Retry":       []string{"2"},
		"X-Delay":       []string{"1500"},
		"Nats-Msg-Id":   []string{"server-id"},
		"nAtS-Schedule": []string{"server-schedule"},
	}

	headers := headersFromNATS(source)
	want := contractqueue.Headers{
		"trace-id":                []string{"request-1"},
		"tags":                    []string{"email", "urgent"},
		contractqueue.RetryHeader: []string{"2"},
		contractqueue.DelayHeader: []string{"1500"},
	}
	if !reflect.DeepEqual(headers, want) {
		t.Fatalf("headersFromNATS() = %#v, want %#v", headers, want)
	}
	if got := contractqueue.RetryCount(headers); got != 2 {
		t.Fatalf("retry count = %d, want 2", got)
	}
	if got := contractqueue.RetryDelay(headers); got != 1500*time.Millisecond {
		t.Fatalf("retry delay = %s, want %s", got, 1500*time.Millisecond)
	}

	source["tags"][0] = "changed"
	if got := headers["tags"].([]string)[0]; got != "email" {
		t.Fatalf("headersFromNATS() did not copy values: %q", got)
	}
}

func TestRequeueUsesTieredRetryDelay(t *testing.T) {
	options := queueOptions{
		retry: []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute},
	}

	for _, tt := range []struct {
		retried int
		wait    time.Duration
		ok      bool
	}{
		{0, time.Minute, true},
		{1, 5 * time.Minute, true},
		{2, 30 * time.Minute, true},
		{3, 0, false},
	} {
		wait, ok := contractqueue.RetryAfter(options.retry, tt.retried)
		if ok != tt.ok || wait != tt.wait {
			t.Fatalf("retried=%d got (%s, %v), want (%s, %v)", tt.retried, wait, ok, tt.wait, tt.ok)
		}
	}
}

func TestScheduleConfiguration(t *testing.T) {
	options := queueOptions{
		schedulePrefix: "custom.schedule",
		retention:      2 * time.Hour,
	}

	if got := options.scheduleSubject("EVENTS"); got != "custom.schedule.EVENTS" {
		t.Fatalf("schedule subject = %q, want %q", got, "custom.schedule.EVENTS")
	}
	if got := options.delayedMessageTTL(0); got != 2*time.Hour {
		t.Fatalf("default delayed message TTL = %s, want %s", got, 2*time.Hour)
	}
	if got := options.delayedMessageTTL(30 * time.Minute); got != 30*time.Minute {
		t.Fatalf("explicit delayed message TTL = %s, want %s", got, 30*time.Minute)
	}
	if got := options.streamMaxAge(30 * time.Minute); got != 2*time.Hour {
		t.Fatalf("stream max age = %s, want %s", got, 2*time.Hour)
	}
	if got := options.streamMaxAge(3 * time.Hour); got != 3*time.Hour {
		t.Fatalf("stream max age = %s, want %s", got, 3*time.Hour)
	}
}

func TestConsumerTTLConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		current   time.Duration
		requested time.Duration
		want      time.Duration
	}{
		{name: "new TTL", requested: time.Minute, want: time.Minute},
		{name: "shorter TTL", current: time.Hour, requested: time.Minute, want: time.Minute},
		{name: "keep shorter TTL", current: time.Minute, requested: time.Hour, want: time.Minute},
		{name: "keep configured TTL", current: time.Minute, want: time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minimumTTL(tt.current, tt.requested); got != tt.want {
				t.Fatalf("minimumTTL() = %s, want %s", got, tt.want)
			}
		})
	}

	if got := streamConsumerTTL(map[string]string{consumerTTLStreamMetadata: "30s"}); got != 30*time.Second {
		t.Fatalf("stream consumer TTL = %s, want %s", got, 30*time.Second)
	}
	if got := streamConsumerTTL(map[string]string{consumerTTLStreamMetadata: "invalid"}); got != 0 {
		t.Fatalf("invalid stream consumer TTL = %s, want 0", got)
	}
}

func TestConsumerName(t *testing.T) {
	name := consumerName("workers", "events.created")
	if name != consumerName("workers", "events.created") {
		t.Fatal("consumer name is not deterministic")
	}
	if name == consumerName("workers", "events.updated") {
		t.Fatal("different subjects produced the same consumer name")
	}
	if strings.ContainsAny(name, ".*>/\\ ") {
		t.Fatalf("consumer name contains invalid characters: %q", name)
	}
}

func TestAcknowledge(t *testing.T) {
	tests := []struct {
		name     string
		response any
		want     Action
		wantErr  bool
	}{
		{name: "default", want: Ack},
		{name: "ack", response: Ack, want: Ack},
		{name: "nak", response: Nak, want: Nak},
		{name: "term", response: Term, want: Term},
		{name: "invalid type", response: "ack", want: Nak, wantErr: true},
		{name: "invalid action", response: Action(99), want: Nak, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &fakeAcknowledgement{}
			err := acknowledge(message, tt.response)
			if (err != nil) != tt.wantErr {
				t.Fatalf("acknowledge() error = %v, wantErr %t", err, tt.wantErr)
			}
			if message.action != tt.want {
				t.Fatalf("acknowledge() action = %v, want %v", message.action, tt.want)
			}
		})
	}

	wantErr := errors.New("ack failed")
	message := &fakeAcknowledgement{err: wantErr}
	if err := acknowledge(message, nil); !errors.Is(err, wantErr) {
		t.Fatalf("acknowledge() error = %v, want %v", err, wantErr)
	}
}
