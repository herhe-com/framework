package rabbitmq

import (
	"sync"

	"github.com/gookit/color"
	amqp "github.com/rabbitmq/amqp091-go"
)

// exchangeDecl describes an exchange a consumer declares and the routing keys
// the consumed queue is bound to.
type exchangeDecl struct {
	name     string
	kind     string
	durable  bool
	declare  bool
	args     amqp.Table
	bindings []string
}

// consumerSpec is everything needed to (re)start one consumer.
type consumerSpec struct {
	queue       string
	durable     bool
	queueArgs   amqp.Table
	exchanges   []exchangeDecl
	prefetch    int
	workers     int
	handler     func(amqp.Delivery) Action
	consumerTag string
}

// consumer owns one channel, its delivery subscription and a worker pool.
//
// Short disconnects are handled by amqp091's own recovery, which re-opens the
// channel, re-declares topology and re-subscribes with the same delivery
// channel, so the workers below keep running untouched. This type only restarts
// things when the driver has to build a brand new connection (native recovery
// gave up), which is why the previous workers must be drained first.
type consumer struct {
	conn *connection
	spec consumerSpec

	mu     sync.Mutex
	ch     *amqp.Channel
	closed bool

	wg   sync.WaitGroup
	done chan struct{}
}

func newConsumer(conn *connection, spec consumerSpec) (*consumer, error) {
	if spec.workers <= 0 {
		spec.workers = 1
	}

	if conn == nil {
		return nil, ErrDisconnected
	}

	c := &consumer{conn: conn, spec: spec, done: make(chan struct{})}

	if err := conn.register(c); err != nil {
		return nil, err
	}

	return c, nil
}

// recover rebuilds the subscription on a fresh connection.
func (c *consumer) recover(conn *amqp.Connection) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ErrClosed
	}

	// The previous generation must be fully gone before consuming again,
	// otherwise two consumers briefly share the queue and messages are
	// delivered twice.
	c.stopWorkers()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	if err = c.declare(ch); err != nil {
		_ = ch.Close()

		return err
	}

	deliveries, err := ch.Consume(c.spec.queue, c.spec.consumerTag, false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()

		return err
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = ch.Close()

		return ErrClosed
	}
	c.ch = ch
	c.mu.Unlock()

	c.wg.Add(c.spec.workers)
	for range c.spec.workers {
		go c.work(deliveries)
	}

	return nil
}

// declare applies QoS and the queue/exchange topology of this consumer.
func (c *consumer) declare(ch *amqp.Channel) error {
	if c.spec.prefetch > 0 {
		if err := ch.Qos(c.spec.prefetch, 0, false); err != nil {
			return err
		}
	}

	for _, exchange := range c.spec.exchanges {
		if !exchange.declare {
			continue
		}
		err := ch.ExchangeDeclare(
			exchange.name,
			exchange.kind,
			exchange.durable,
			false,
			false,
			false,
			exchange.args,
		)
		if err != nil {
			return err
		}
	}

	if _, err := ch.QueueDeclare(c.spec.queue, c.spec.durable, false, false, false, c.spec.queueArgs); err != nil {
		return err
	}

	for _, exchange := range c.spec.exchanges {
		for _, route := range exchange.bindings {
			if err := ch.QueueBind(c.spec.queue, route, exchange.name, false, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

// work consumes one worker's share of the deliveries. Closing the channel ends
// the range loop, which is how both a cancel and a dead connection are seen.
func (c *consumer) work(deliveries <-chan amqp.Delivery) {
	defer c.wg.Done()

	for delivery := range deliveries {
		c.settle(delivery, c.spec.handler(delivery))
	}
}

// settle applies the action the handler chose to the delivery.
func (c *consumer) settle(delivery amqp.Delivery, action Action) {
	var err error

	switch action {
	case Ack:
		err = delivery.Ack(false)
	case NackDiscard:
		err = delivery.Nack(false, false)
	default:
		err = delivery.Nack(false, true)
	}

	if err != nil && !isClosedError(err) {
		color.Errorf("[queue.rabbitmq] acknowledge message %s: %v\n", delivery.MessageId, err)
	}
}

// stopWorkers cancels the subscription, closes its channel and waits for every
// worker to return.
func (c *consumer) stopWorkers() {
	c.mu.Lock()
	ch := c.ch
	c.ch = nil
	c.mu.Unlock()

	// Closing the channel cancels the subscription and closes the delivery
	// channel, which unblocks the workers.
	if ch != nil {
		if err := ch.Close(); err != nil && !isClosedError(err) {
			color.Errorf("[queue.rabbitmq] close consumer channel: %v\n", err)
		}
	}

	c.wg.Wait()
}

// shutdown stops the consumer for good and unblocks wait.
func (c *consumer) shutdown() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()

	c.stopWorkers()
	close(c.done)
}

// wait blocks until the consumer is shut down, mirroring the previous driver
// where Consumer() ran until the connection was closed.
func (c *consumer) wait() {
	<-c.done
}
