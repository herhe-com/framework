package rabbitmq

import (
	"context"
	"sync"

	"github.com/gookit/color"
	amqp "github.com/rabbitmq/amqp091-go"
)

// publishSpec describes the channel a publisher owns and the exchange it
// declares. Channels are cached per destination, so one queue never opens a
// channel per message.
type publishSpec struct {
	exchange string
	kind     string
	durable  bool
	declare  bool
	args     amqp.Table
	// onReturn receives publishes the broker could not route (mandatory).
	onReturn func(amqp.Return)
}

// publisher owns one channel on the shared connection and reopens it lazily
// when the connection or the channel itself was lost.
type publisher struct {
	conn *connection
	spec publishSpec

	mu  sync.Mutex
	ch  *amqp.Channel
	gen uint64

	// publishMu serialises publishes: an AMQP message spans several frames and
	// amqp091 only locks individual frame writes, so concurrent publishes on one
	// channel would interleave and corrupt the stream.
	publishMu sync.Mutex
}

// newPublisher creates the publisher and opens its channel right away, so an
// invalid exchange declaration fails at registration time instead of on the
// first message.
func newPublisher(conn *connection, spec publishSpec) (*publisher, error) {
	if conn == nil {
		return nil, ErrDisconnected
	}

	publisher := &publisher{conn: conn, spec: spec}

	if _, err := publisher.channel(); err != nil {
		return nil, err
	}

	return publisher, nil
}

// channel returns a channel of the current connection, reopening it when the
// cached one belongs to a dead connection or was closed by a channel error.
func (p *publisher) channel() (*amqp.Channel, error) {
	p.mu.Lock()
	cached, gen := p.ch, p.gen
	p.mu.Unlock()

	conn, current, err := p.conn.current()
	if err != nil {
		return nil, err
	}
	if cached != nil && gen == current && !cached.IsClosed() {
		return cached, nil
	}

	ch, err := p.open(conn)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	if p.ch != nil && p.gen == current {
		// A concurrent opener won the race; keep its channel and drop ours.
		winner := p.ch
		p.mu.Unlock()
		_ = ch.Close()

		return winner, nil
	}
	stale := p.ch
	p.ch, p.gen = ch, current
	p.mu.Unlock()

	if stale != nil {
		_ = stale.Close()
	}

	return ch, nil
}

// open creates a channel and declares the exchange on it.
func (p *publisher) open(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	if p.spec.declare {
		err = ch.ExchangeDeclare(
			p.spec.exchange,
			p.spec.kind,
			p.spec.durable,
			false,
			false,
			false,
			p.spec.args,
		)
		if err != nil {
			_ = ch.Close()

			return nil, err
		}
	}

	if p.spec.onReturn != nil {
		returns := ch.NotifyReturn(make(chan amqp.Return, 1))

		// The channel closes on reconnect, which closes this channel and ends
		// the goroutine.
		go func() {
			for returned := range returns {
				p.spec.onReturn(returned)
			}
		}()
	}

	return ch, nil
}

// publish writes one message per routing key. Failures are returned to the
// caller: a retry path can nack the delivery and let the broker redeliver it
// once the connection is back.
func (p *publisher) publish(
	ctx context.Context,
	exchange string,
	routes []string,
	mandatory bool,
	message amqp.Publishing,
) error {
	ch, err := p.channel()
	if err != nil {
		return err
	}

	p.publishMu.Lock()
	defer p.publishMu.Unlock()

	for _, route := range routes {
		if err = ch.PublishWithContext(ctx, exchange, route, mandatory, false, message); err != nil {
			return err
		}
	}

	return nil
}

// shutdown closes the channel. Publishers recover lazily, so no restart hook is
// needed: the next publish opens a channel on the live connection.
func (p *publisher) shutdown() {
	p.mu.Lock()
	ch := p.ch
	p.ch = nil
	p.mu.Unlock()

	if ch == nil {
		return
	}

	if err := ch.Close(); err != nil && !isClosedError(err) {
		color.Errorf("[queue.rabbitmq] close publisher channel: %v\n", err)
	}
}
