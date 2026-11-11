package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// keepAlive refreshes the acknowledgement deadline while a handler runs.
// JetStream redelivers a message once AckWait elapses, so a handler that runs
// longer than AckWait would otherwise be executed again in parallel with the
// original delivery.
func keepAlive(message jetstream.Msg, ackWait time.Duration) func() {
	if ackWait <= 0 {
		return func() {}
	}

	interval := ackWait / 3
	if interval <= 0 {
		interval = ackWait / 2
	}

	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := message.InProgress(); err != nil {
					return
				}
			}
		}
	}()

	return func() { close(done) }
}

type acknowledgeMessage interface {
	Ack() error
	Nak() error
	Term() error
}

func acknowledge(message acknowledgeMessage, response any) error {
	if response == nil {
		return message.Ack()
	}

	action, ok := response.(Action)
	if !ok {
		if err := message.Nak(); err != nil {
			return fmt.Errorf("nats: invalid consumer response %T: %w", response, err)
		}
		return fmt.Errorf("nats: invalid consumer response %T; message negatively acknowledged", response)
	}

	switch action {
	case Ack:
		return message.Ack()
	case Nak:
		return message.Nak()
	case Term:
		return message.Term()
	default:
		if err := message.Nak(); err != nil {
			return fmt.Errorf("nats: invalid consumer action %d: %w", action, err)
		}
		return fmt.Errorf("nats: invalid consumer action %d; message negatively acknowledged", action)
	}
}
