package rabbitmq

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	amqp "github.com/rabbitmq/amqp091-go"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
)

// DelayQueuePrefix namespaces the TTL waiting rooms that implement delayed
// publishing when the broker has no delayed message plugin.
//
// A waiting room is a durable queue with x-message-ttl that dead-letters back
// into the business exchange using x-dead-letter-routing-key. Publishing into
// it through the default exchange therefore delays a message by exactly the
// queue TTL, and the business exchange keeps its original type.
const DelayQueuePrefix = "_framework.queue.delay"

// DelayExchange is the shared x-delayed-message exchange used when the broker
// supports the community delayed message plugin (RabbitMQ 4.2 and earlier with
// the plugin enabled). It deliberately shares the DelayQueuePrefix value: the
// name is unchanged from previous releases so existing clusters keep working,
// and the two never occupy the same object type.
const DelayExchange = DelayQueuePrefix

// delayMode is how delays are implemented against the connected broker.
type delayMode int

const (
	// delayModeUndecided means the broker has not been probed yet (auto mode).
	delayModeUndecided delayMode = iota
	// delayModePlugin uses the x-delayed-message exchange of the community
	// delayed message plugin.
	delayModePlugin
	// delayModeTTL uses TTL waiting queues plus dead-lettering, which works on
	// every RabbitMQ version and needs no plugin.
	delayModeTTL
)

// String returns the human readable delay mode name.
func (m delayMode) String() string {
	switch m {
	case delayModePlugin:
		return "plugin (x-delayed-message)"
	case delayModeTTL:
		return "ttl (dead-letter)"
	default:
		return "undecided"
	}
}

// parseDelayMode maps the rabbitmq.delay_mode configuration value.
func parseDelayMode(value string) (delayMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return delayModeUndecided, nil
	case "plugin", "delayed", "x-delayed-message":
		return delayModePlugin, nil
	case "ttl", "dead-letter":
		return delayModeTTL, nil
	default:
		return delayModeUndecided, fmt.Errorf(
			"rabbitmq: unknown delay_mode %q, want auto, plugin or ttl",
			value,
		)
	}
}

// declarer owns the channel used for capability probing and for declaring the
// TTL waiting rooms, which no consumer consumes from.
type declarer struct {
	conn   *connection
	forced delayMode

	mu       sync.Mutex
	ch       *amqp.Channel
	gen      uint64
	resolved delayMode
	modeGen  uint64
	queues   map[string]struct{}
}

// channelLocked is channel with d.mu held.
func (d *declarer) channelLocked() (*amqp.Channel, error) {
	conn, gen, err := d.conn.current()
	if err != nil {
		return nil, err
	}

	if d.ch != nil && d.gen == gen && !d.ch.IsClosed() {
		return d.ch, nil
	}
	if d.ch != nil {
		_ = d.ch.Close()
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	d.ch, d.gen = ch, gen

	return ch, nil
}

// mode resolves the delay implementation once per connection. In auto mode it
// probes the broker instead of parsing its version: the delayed message plugin
// is community maintained, so a 4.2 server may lack it while a rebuilt 4.3
// server may ship it.
func (d *declarer) mode() (delayMode, error) {
	if d.forced != delayModeUndecided {
		return d.forced, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	conn, gen, err := d.conn.current()
	if err != nil {
		return delayModeUndecided, err
	}
	if d.resolved != delayModeUndecided && d.modeGen == gen {
		return d.resolved, nil
	}

	ch, err := d.channelLocked()
	if err != nil {
		return delayModeUndecided, err
	}

	version := "unknown version"
	if advertised, ok := conn.Properties["version"].(string); ok && advertised != "" {
		version = advertised
	}

	// The broker rejects an unknown exchange type, which is exactly the
	// capability delayed publishing needs.
	mode := delayModePlugin

	err = ch.ExchangeDeclare(
		DelayExchange,
		"x-delayed-message",
		true,
		false,
		false,
		false,
		amqp.Table{"x-delayed-type": amqp.ExchangeDirect},
	)
	if err != nil {
		// The rejected declaration closed the channel; drop it so the next
		// declaration reconnects.
		if d.ch != nil {
			_ = d.ch.Close()
			d.ch = nil
		}

		// Only a rejected exchange type means the plugin is missing. Any other
		// failure (network, permissions, argument conflict) must not silently pin
		// the connection to the TTL path. See isUnknownExchangeType.
		if !isUnknownExchangeType(err) {
			return delayModeUndecided, fmt.Errorf("rabbitmq: probe delay exchange: %w", err)
		}

		mode = delayModeTTL
	}

	d.resolved, d.modeGen = mode, gen

	color.Infof("[queue.rabbitmq] rabbitmq %s: delay mode %s\n", version, mode)

	return mode, nil
}

// isUnknownExchangeType reports whether the broker rejected a declaration
// because it does not know the x-delayed-message type.
//
// RabbitMQ 4.3+ answers 406 (precondition failed) with "unknown exchange type
// 'x-delayed-message'" because the community plugin is gone; older brokers
// answer 503 (command invalid) or 540 (not implemented). Every other error,
// including a 406 for inequivalent arguments, is a real failure that must not
// be mistaken for "plugin not installed".
func isUnknownExchangeType(err error) bool {
	var amqpErr *amqp.Error
	if !errors.As(err, &amqpErr) {
		return false
	}

	switch amqpErr.Code {
	case amqp.CommandInvalid, amqp.NotImplemented:
		return true
	case amqp.PreconditionFailed:
		return strings.Contains(strings.ToLower(amqpErr.Reason), "unknown exchange type")
	}

	return false
}

// ensure declares the waiting room of one (topic, route, wait) triple unless
// this driver already did.
func (d *declarer) ensure(topic, route string, wait time.Duration) (delayQueue, error) {
	queue := newDelayQueue(topic, route, wait)

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.queues[queue.name]; exists {
		return queue, nil
	}

	ch, err := d.channelLocked()
	if err != nil {
		return delayQueue{}, err
	}

	_, err = ch.QueueDeclare(queue.name, true, false, false, false, amqp.Table{
		"x-message-ttl":             queue.ttl.Milliseconds(),
		"x-dead-letter-exchange":    queue.exchange,
		"x-dead-letter-routing-key": queue.routingKey,
	})
	if err != nil {
		// A failed declaration closes the channel (a queue that already exists
		// with different arguments, for example), so drop it and let the next
		// attempt reconnect.
		if d.ch != nil {
			_ = d.ch.Close()
			d.ch = nil
		}

		return delayQueue{}, fmt.Errorf("rabbitmq: declare delay queue %s: %w", queue.name, err)
	}

	if d.queues == nil {
		d.queues = make(map[string]struct{})
	}
	d.queues[queue.name] = struct{}{}

	return queue, nil
}

// forget drops a cached declaration so the next publish redeclares the waiting
// room. It runs when the broker returns a publish as unroutable, which means the
// queue disappeared underneath us.
func (d *declarer) forget(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.queues, name)
}

// close releases the declaration channel.
func (d *declarer) close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ch != nil {
		_ = d.ch.Close()
		d.ch = nil
	}

	d.queues = nil
}

// delayQueue describes one TTL waiting room.
type delayQueue struct {
	name       string
	ttl        time.Duration
	exchange   string
	routingKey string
}

// newDelayQueue builds the waiting room of one (topic, route, wait) triple. The
// name and the TTL share the same millisecond value so a queue never holds
// messages with different deadlines. A uniform TTL per queue keeps expiry FIFO
// and avoids the head-of-line blocking of per-message TTL.
func newDelayQueue(topic, route string, wait time.Duration) delayQueue {
	milliseconds := delayMilliseconds(wait)

	return delayQueue{
		name:       delayQueueName(topic, route, milliseconds),
		ttl:        time.Duration(milliseconds) * time.Millisecond,
		exchange:   topic,
		routingKey: route,
	}
}

// delayMilliseconds rounds a wait up to whole milliseconds so the queue TTL
// never fires before the requested delay.
func delayMilliseconds(wait time.Duration) int64 {
	if wait <= 0 {
		return 0
	}

	return int64((wait + time.Millisecond - 1) / time.Millisecond)
}

// delayQueueName derives a stable, readable queue name. Names that need
// sanitization or that are too long fall back to a hash so that two different
// routing targets can never collide on one queue with mismatched declaration
// arguments.
func delayQueueName(topic, route string, milliseconds int64) string {
	raw := fmt.Sprintf("%s.%d.%s.%s", DelayQueuePrefix, milliseconds, topic, route)
	name := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case character == '.', character == '-', character == '_':
			return character
		default:
			return '_'
		}
	}, raw)

	if name == raw && len(name) <= 200 {
		return name
	}

	sum := sha256.Sum256([]byte(raw))

	return fmt.Sprintf("%s.%d.%x", DelayQueuePrefix, milliseconds, sum[:6])
}

// onUndeliverable reports a delayed message the broker could not route and
// invalidates the cached declaration so the next publish recreates the waiting
// room instead of dropping messages again.
//
// The publish that triggered this already returned success: a returned message
// is gone, so delayed publishing through the TTL waiting rooms is at-most-once.
func (r *RabbitMQ) onUndeliverable(ret amqp.Return) {
	r.declarer.forget(ret.RoutingKey)

	color.Errorf(
		"[queue.rabbitmq] delayed publish to %s returned (%d %s); the message is lost (at-most-once) and the waiting room is recreated on the next publish\n",
		ret.RoutingKey,
		ret.ReplyCode,
		ret.ReplyText,
	)
}

// publishDelayed delays a first produce by options.delay. With the plugin the
// business exchange is itself x-delayed-message, so the message is published
// straight to it; without the plugin it waits in a TTL queue that dead-letters
// it back.
func (r *RabbitMQ) publishDelayed(
	data []byte,
	options queueOptions,
	routes []string,
	headers contractqueue.Headers,
) error {
	if err := r.CheckQueue(options.queue); err != nil {
		return err
	}

	if options.mode == delayModePlugin {
		publisher, err := r.publisher(options.publisherKey(), r.publishSpec(options))
		if err != nil {
			return err
		}

		return r.publishPlugin(publisher, options.topic, data, routes, options, headers, options.delay)
	}

	return r.publishDelayedTTL(data, options, routes, headers, options.delay)
}

// publishRetry requeues a failed message after wait. Retries never change the
// business exchange: with the plugin they go through the shared delay exchange,
// without it through a TTL waiting room.
func (r *RabbitMQ) publishRetry(
	data []byte,
	options queueOptions,
	route string,
	headers contractqueue.Headers,
	wait time.Duration,
) error {
	if wait <= 0 {
		return r.publish(data, options, []string{route}, headers)
	}

	if err := r.CheckQueue(options.queue); err != nil {
		return err
	}

	if options.mode == delayModePlugin {
		publisher, err := r.publisher(delayPluginPublisherKey, delayPluginPublishSpec)
		if err != nil {
			return err
		}

		return r.publishPlugin(publisher, DelayExchange, data, []string{route}, options, headers, wait)
	}

	return r.publishDelayedTTL(data, options, []string{route}, headers, wait)
}

// delayHeaders copies the business headers and stamps the delay the plugin
// exchange reads once.
func delayHeaders(headers contractqueue.Headers, wait time.Duration) amqp.Table {
	header := make(amqp.Table, len(headers)+1)
	for key, value := range headers {
		header[key] = value
	}
	header[contractqueue.DelayHeader] = wait.Milliseconds()

	return header
}

// publishPlugin publishes through an x-delayed-message exchange, which holds the
// message until the x-delay header elapses and then routes it by routing key.
func (r *RabbitMQ) publishPlugin(
	publisher *publisher,
	exchange string,
	data []byte,
	routes []string,
	options queueOptions,
	headers contractqueue.Headers,
	wait time.Duration,
) error {
	header := delayHeaders(headers, wait)

	message := r.publishing(options, data, nil)
	message.Headers = header

	return publisher.publish(context.Background(), exchange, routes, false, message)
}

// publishDelayedTTL routes data into one waiting room per routing key. Messages
// dead-letter back into options.topic with the original routing key, so
// consumers keep seeing what looks like a plain publish.
//
// At-most-once: a mandatory publish that the broker cannot route (for example
// because the waiting room was deleted) is reported by onUndeliverable and
// dropped after Producer already returned success. Callers that need a
// guarantee should verify the topology or use the plugin delay mode.
func (r *RabbitMQ) publishDelayedTTL(
	data []byte,
	options queueOptions,
	routes []string,
	headers contractqueue.Headers,
	wait time.Duration,
) error {
	// Creating the business publisher also declares the exchange the waiting
	// rooms dead-letter into: RabbitMQ silently drops messages dead-lettered to
	// a missing exchange.
	if _, err := r.publisher(options.publisherKey(), r.publishSpec(options)); err != nil {
		return err
	}

	publisher, err := r.publisher(defaultPublisherKey, publishSpec{onReturn: r.onUndeliverable})
	if err != nil {
		return err
	}

	header := delayHeaders(headers, wait)

	// The queue's ttl intentionally skips the expiration property: expiring
	// inside the waiting room would dead-letter the message early.
	options.ttl = 0
	message := r.publishing(options, data, nil)
	message.Headers = header

	for _, route := range routes {
		queue, err := r.declarer.ensure(options.topic, route, wait)
		if err != nil {
			return err
		}

		// The default exchange routes on the queue name, so the message lands in
		// the waiting room even though its business routing key differs. Mandatory
		// publishing turns a missing waiting room into a returned message.
		if err = publisher.publish(context.Background(), "", []string{queue.name}, true, message); err != nil {
			return err
		}
	}

	return nil
}

// delayPluginPublisherKey identifies the shared publisher of the plugin delay
// exchange. Queue publishers always carry a configuration key, so they cannot
// collide with it.
var delayPluginPublisherKey = publisherKey{topic: DelayExchange}

// delayPluginPublishSpec declares the shared x-delayed-message exchange used
// when the broker has the delayed message plugin. x-delayed-type is direct so
// consumers bind their queue to the exchange with plain routing keys.
var delayPluginPublishSpec = publishSpec{
	exchange: DelayExchange,
	kind:     "x-delayed-message",
	durable:  true,
	declare:  true,
	args:     amqp.Table{"x-delayed-type": amqp.ExchangeDirect},
}
