package rabbitmq

import (
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/gookit/color"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrDisconnected is returned when an operation needs a live broker connection
// while the driver is reconnecting. Callers can retry once it is back.
var ErrDisconnected = errors.New("rabbitmq: connection is unavailable, reconnecting")

// recoverer rebuilds channel-level state on a fresh connection. Publishers and
// the delay declarer recover lazily, so only consumers implement it.
type recoverer interface {
	// recover rebuilds state on conn. It is called once per established
	// connection, before any new traffic uses it.
	recover(conn *amqp.Connection) error
	// shutdown releases local resources when the driver closes.
	shutdown()
}

// connection owns the AMQP connection and re-establishes it after a failure.
//
// Every channel is bound to a connection, so a reconnect invalidates all of
// them at once. The generation counter lets lazily opened channels detect that
// they belong to a dead connection, while recoverers are replayed eagerly so
// consumers resume without waiting for traffic.
type connection struct {
	url    string
	label  string
	config amqp.Config
	base   time.Duration
	max    time.Duration

	mu         sync.RWMutex
	conn       *amqp.Connection
	gen        uint64
	open       bool
	recoverers []recoverer
	closed     bool

	// buildMu serialises recoverer startup so a registration racing with a
	// reconnect cannot start the same consumer twice.
	buildMu sync.Mutex

	stop chan struct{}
	done chan struct{}
}

// newConnection dials the broker and starts watching the connection. It returns
// an error when the initial dial fails, so misconfiguration surfaces at startup
// instead of being retried silently.
func newConnection(url, label string, config amqp.Config, base time.Duration) (*connection, error) {
	if base <= 0 {
		base = 3 * time.Second
	}

	conn, err := amqp.DialConfig(url, config)
	if err != nil {
		return nil, err
	}

	c := &connection{
		url:    url,
		label:  label,
		config: config,
		base:   base,
		max:    base * 16,
		conn:   conn,
		gen:    1,
		open:   true,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}

	c.observe(conn)

	go c.watch(conn)

	return c, nil
}

// current returns the live connection and its generation, or ErrClosed /
// ErrDisconnected when there is nothing to publish on.
//
// A connection that is mid-recovery is reported as disconnected: opening a
// channel while the client is still handshaking interleaves a frame with the
// handshake and gets the connection rejected by the broker.
func (c *connection) current() (*amqp.Connection, uint64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, 0, ErrClosed
	}
	if c.conn == nil || !c.open {
		return nil, 0, ErrDisconnected
	}

	return c.conn, c.gen, nil
}

// observe tracks the lifecycle of conn so that current() only reports a
// connection that finished its handshake and topology recovery.
func (c *connection) observe(conn *amqp.Connection) {
	states := make(chan *amqp.StateChanged, 8)
	conn.NotifyStateChange(states)

	go func() {
		for state := range states {
			switch state.To {
			case amqp.StateOpen:
				c.setOpen(conn, true)
				if state.From == amqp.StateReconnecting {
					color.Infof("[queue.rabbitmq] connection %s recovered\n", c.label)
				}
			case amqp.StateReconnecting:
				c.setOpen(conn, false)
				color.Warnf("[queue.rabbitmq] connection %s reconnecting: %v\n", c.label, state.Err)
			case amqp.StateClosed:
				c.setOpen(conn, false)
			}
		}
	}()
}

// setOpen records the state of conn, ignoring transitions of a connection that
// was already replaced by the watchdog.
func (c *connection) setOpen(conn *amqp.Connection, open bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != conn {
		return
	}

	c.open = open
}

// register adds a recoverer and starts it on the current connection. A start
// failure removes it again so a broken topology does not get retried forever.
func (c *connection) register(item recoverer) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return ErrDisconnected
	}
	c.recoverers = append(c.recoverers, item)
	c.mu.Unlock()

	c.buildMu.Lock()
	err := item.recover(conn)
	c.buildMu.Unlock()

	if err != nil {
		c.mu.Lock()
		for index, registered := range c.recoverers {
			if registered == item {
				c.recoverers = slices.Delete(c.recoverers, index, index+1)
				break
			}
		}
		c.mu.Unlock()
	}

	return err
}

// watch reconnects after every failure until the driver closes.
func (c *connection) watch(conn *amqp.Connection) {
	defer close(c.done)

	for {
		if !c.waitClose(conn) {
			return
		}

		next := c.reconnect()
		if next == nil {
			return
		}

		conn = next
	}
}

// waitClose blocks until the connection is gone for good. It returns false when
// the driver is closing and no reconnect should be attempted.
//
// NotifyClose reports every disconnect, including the ones the client's own
// recovery is already fixing, so only a closed listener channel means the
// recovery gave up and the driver has to rebuild the connection itself.
func (c *connection) waitClose(conn *amqp.Connection) bool {
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-c.stop:
			return false
		case err, ok := <-closed:
			if c.isClosed() {
				return false
			}
			if !ok {
				color.Errorf("[queue.rabbitmq] connection %s lost; rebuilding\n", c.label)

				return true
			}

			color.Warnf(
				"[queue.rabbitmq] connection %s interrupted: %v; waiting for client recovery\n",
				c.label,
				err,
			)
		}
	}
}

// reconnect retries with exponential backoff until it installs a fresh
// connection, or returns nil when the driver closes while waiting.
func (c *connection) reconnect() *amqp.Connection {
	backoff := c.base

	for {
		if !c.sleep(backoff) {
			return nil
		}

		conn, err := amqp.DialConfig(c.url, c.config)
		if err != nil {
			color.Errorf("[queue.rabbitmq] reconnect failed: %v\n", err)
			backoff = c.nextBackoff(backoff)

			continue
		}

		if !c.install(conn) {
			_ = conn.Close()

			return nil
		}

		color.Infof("[queue.rabbitmq] reconnected to %s\n", c.label)

		return conn
	}
}

// install swaps in a fresh connection and replays every recoverer. It reports
// false when the driver closed in the meantime.
func (c *connection) install(conn *amqp.Connection) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	previous := c.conn
	c.conn = conn
	c.gen++
	c.open = true
	recoverers := slices.Clone(c.recoverers)
	c.mu.Unlock()

	// Channels of the dead connection are already gone; closing it releases any
	// handle that outlived the broker socket.
	if previous != nil && previous != conn {
		_ = previous.Close()
	}

	c.observe(conn)

	c.buildMu.Lock()
	defer c.buildMu.Unlock()

	for _, item := range recoverers {
		if err := item.recover(conn); err != nil {
			// Keep the recoverer registered and try again on the next reconnect,
			// matching the behaviour of a consumer that lost its channel.
			color.Errorf("[queue.rabbitmq] recovery after reconnect failed: %v\n", err)
		}
	}

	return true
}

func (c *connection) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.closed
}

func (c *connection) sleep(duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-c.stop:
		return false
	case <-timer.C:
		return true
	}
}

// nextBackoff doubles the interval up to max and adds jitter so that a fleet of
// workers does not retry in lockstep after a broker restart.
func (c *connection) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > c.max {
		next = c.max
	}

	delta := float64(next) * 0.25

	return time.Duration(float64(next) + (rand.Float64()*2-1)*delta)
}

// Close stops the reconnect loop, shuts down every recoverer and closes the
// broker connection. It is safe to call more than once.
func (c *connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.open = false
	conn := c.conn
	c.conn = nil
	recoverers := slices.Clone(c.recoverers)
	c.recoverers = nil
	c.mu.Unlock()

	close(c.stop)

	// Consumers are stopped before the connection closes so their workers can
	// finish in-flight handlers and their goroutines exit.
	c.buildMu.Lock()
	for _, item := range recoverers {
		item.shutdown()
	}
	c.buildMu.Unlock()

	<-c.done

	if conn == nil {
		return nil
	}

	return conn.Close()
}
