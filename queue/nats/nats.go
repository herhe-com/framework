package nats

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/viper"

	contractqueue "github.com/herhe-com/framework/contracts/queue"
	queueconfig "github.com/herhe-com/framework/queue/config"
)

const (
	managedStreamMetadata     = "framework.queue.managed"
	consumerTTLStreamMetadata = "framework.queue.consumer_ttl"

	// defaultAckWait is JetStream's own acknowledgement window.
	defaultAckWait = 30 * time.Second
	// defaultMaxAckPending keeps a single message in flight. Handlers run one at
	// a time, and JetStream starts the acknowledgement clock when a message is
	// delivered, so a larger window only queues messages that expire before the
	// handler reaches them and get handled twice.
	defaultMaxAckPending = 1
)

// Action controls how a JetStream message is acknowledged.
type Action int

const (
	// Ack acknowledges a successfully processed message.
	Ack Action = iota
	// Nak asks JetStream to redeliver the message.
	Nak
	// Term stops redelivery of the message.
	Term
)

type streamBinding struct {
	stream      string
	schedule    string
	maxAge      time.Duration
	consumerTTL time.Duration
}

// NATS implements core NATS queues and JetStream-backed delayed and TTL queues.
type NATS struct {
	conn     *natsgo.Conn
	js       jetstream.JetStream
	cfg      *viper.Viper
	name     string
	done     chan struct{}
	doneOnce sync.Once
	streamMu sync.Mutex
	streams  map[string]streamBinding
}

type queueOptions struct {
	topic          string
	queue          string
	routes         []string
	delay          time.Duration
	ttl            time.Duration
	delayed        bool
	retry          []time.Duration
	headers        contractqueue.Headers
	errorEnabled   bool
	errorSubject   string
	stream         string
	schedulePrefix string
	retention      time.Duration
	ackWait        time.Duration
	maxAckPending  int
	handlerTimeout time.Duration
}

var _ contractqueue.Driver = (*NATS)(nil)

// NewNATS creates and connects a NATS queue driver.
func NewNATS(configs map[string]any, names ...string) (*NATS, error) {
	name := ""
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		name = strings.TrimSpace(names[0])
	}

	cfg := viper.New()
	cfg.Set("nats", configs)

	driver := &NATS{
		cfg:     cfg,
		name:    name,
		done:    make(chan struct{}),
		streams: make(map[string]streamBinding),
	}

	conn, err := natsgo.Connect(driver.url(), driver.options()...)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	driver.conn = conn
	driver.js = js
	conn.SetClosedHandler(func(*natsgo.Conn) {
		driver.signalDone()
	})
	if conn.IsClosed() {
		driver.signalDone()
	}

	endpoint := conn.ConnectedUrlRedacted()
	if endpoint == "" {
		endpoint = driver.url()
	}

	color.Infof(
		"[queue.nats] connection %s connected to %s (server %s)\n",
		driver.label(),
		endpoint,
		conn.ConnectedServerVersion(),
	)

	return driver, nil
}

// label returns the connection name used in logs.
func (r *NATS) label() string {
	if r.name != "" {
		return r.name
	}

	return "default"
}

func (r *NATS) url() string {
	if serverURL := r.cfg.GetString("nats.url"); serverURL != "" {
		return serverURL
	}

	host := r.cfg.GetString("nats.host")
	if host == "" {
		host = "127.0.0.1"
	}

	port := r.cfg.GetInt("nats.port")
	if port == 0 {
		port = 4222
	}

	return "nats://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func (r *NATS) options() []natsgo.Option {
	var options []natsgo.Option

	if name := r.cfg.GetString("nats.name"); name != "" {
		options = append(options, natsgo.Name(name))
	}

	if credentials := r.cfg.GetString("nats.credentials"); credentials != "" {
		return append(options, natsgo.UserCredentials(credentials))
	}

	if token := r.cfg.GetString("nats.token"); token != "" {
		return append(options, natsgo.Token(token))
	}

	if username := r.cfg.GetString("nats.username"); username != "" {
		options = append(options, natsgo.UserInfo(username, r.cfg.GetString("nats.password")))
	}

	return options
}

// Producer publishes a message using queue.queues.<key>.
func (r *NATS) Producer(data []byte, key string, headers ...contractqueue.Headers) error {
	options, err := r.queueOptions(key)
	if err != nil {
		return err
	}

	return r.publish(
		data,
		publishSubjects(options.topic, options.queue, options.routes),
		options,
		contractqueue.MergeHeaders(options.headers, contractqueue.MergeHeaders(headers...)),
	)
}

func (r *NATS) publish(
	data []byte,
	subjects []string,
	options queueOptions,
	headers contractqueue.Headers,
) error {
	if r.conn == nil || r.conn.IsClosed() {
		return natsgo.ErrConnectionClosed
	}

	if len(subjects) == 0 {
		return errors.New("nats: subject is required")
	}
	if options.delay > 0 {
		for _, subject := range subjects {
			if err := r.publishDelayed(
				data,
				subject,
				options.delay,
				options.ttl,
				headers,
				options,
			); err != nil {
				return err
			}
		}

		return nil
	}
	if options.ttl > 0 {
		for _, subject := range subjects {
			if err := r.publishWithTTL(data, subject, options.ttl, headers, options); err != nil {
				return err
			}
		}

		return nil
	}

	for _, subject := range subjects {
		message := natsgo.NewMsg(subject)
		message.Data = data
		setHeaders(message, headers)

		if err := r.conn.PublishMsg(message); err != nil {
			return err
		}
	}

	return r.conn.Flush()
}

// Consumer consumes through core NATS or durable JetStream consumers and blocks until Close is called.
func (r *NATS) Consumer(handler contractqueue.Handler, key string) error {
	if handler == nil {
		return errors.New("nats: handler is required")
	}
	if r.conn == nil || r.conn.IsClosed() {
		return natsgo.ErrConnectionClosed
	}

	options, err := r.queueOptions(key)
	if err != nil {
		return err
	}

	subjects := publishSubjects(options.topic, options.queue, options.routes)
	if len(subjects) == 0 {
		return errors.New("nats: subject is required")
	}
	if options.needsSchedules() {
		if err = r.requireScheduleSupport(); err != nil {
			return err
		}
	}

	if options.needsJetStream() {
		return r.consumeJetStream(handler, key, subjects, options)
	}

	callback := func(message *natsgo.Msg) {
		headers := headersFromNATS(message.Header)
		retried := contractqueue.RetryCount(headers)
		_, consumeErr := contractqueue.CallHandler("nats", handler, message.Data, options.handlerTimeout)
		if consumeErr == nil {
			return
		}
		if _, ok := contractqueue.RetryAfter(options.retry, retried); ok {
			if retryErr := r.requeue(message.Data, headers, message.Subject, options, retried); retryErr == nil {
				return
			} else {
				color.Errorf("[queue.nats] requeue message: %v\n", retryErr)
			}
		}

		if options.errorEnabled {
			if publishErr := r.publishError(
				message.Data,
				options,
				message.Subject,
				retried,
				consumeErr,
			); publishErr != nil {
				color.Errorf("[queue.nats] publish error message: %v\n", publishErr)
			}
		}
	}

	for _, subject := range subjects {
		if options.queue == "" {
			_, err = r.conn.Subscribe(subject, callback)
		} else {
			_, err = r.conn.QueueSubscribe(subject, options.queue, callback)
		}
		if err != nil {
			return err
		}
	}

	if err = r.conn.Flush(); err != nil {
		return err
	}

	queueGroup := options.queue
	if queueGroup == "" {
		queueGroup = "-"
	}
	color.Infof(
		"[queue.nats] consumer %s subscribed to %s via core NATS (queue group %s)\n",
		key,
		strings.Join(subjects, ", "),
		queueGroup,
	)

	<-r.done
	return r.conn.LastError()
}

func (r *NATS) publishWithTTL(
	data []byte,
	target string,
	ttl time.Duration,
	headers contractqueue.Headers,
	options queueOptions,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, _, err := r.ensureStream(ctx, target, ttl, false, 0, options); err != nil {
		return err
	}

	message := natsgo.NewMsg(target)
	message.Data = data
	setHeaders(message, headers)

	_, err := r.js.PublishMsg(ctx, message, jetstream.WithMsgTTL(ttl))
	return err
}

func (r *NATS) publishDelayed(
	data []byte,
	target string,
	delay, ttl time.Duration,
	headers contractqueue.Headers,
	options queueOptions,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.requireScheduleSupport(); err != nil {
		return err
	}

	ttl = options.delayedMessageTTL(ttl)
	_, scheduleSubject, err := r.ensureStream(ctx, target, delay+ttl, true, 0, options)
	if err != nil {
		return err
	}

	message := natsgo.NewMsg(scheduleSubject)
	message.Data = data
	setHeaders(message, headers)
	if message.Header == nil {
		message.Header = natsgo.Header{}
	}
	message.Header.Set(
		contractqueue.DelayHeader,
		strconv.FormatInt(delay.Milliseconds(), 10),
	)
	message.Header.Set(jetstream.MsgTTLHeader, "never")
	// The schedule headers are set by hand: nats.go formats `@at` with
	// time.RFC3339, which drops the sub-second part and fires up to a second
	// early.
	//
	// Upgrade checklist: when github.com/nats-io/nats.go is bumped, re-check
	// that jetstream.ScheduleHeader still means `@at` and that nats-server still
	// parses it with time.RFC3339Nano; see queue/README.md. Remove this block
	// once WithScheduleAt keeps the sub-second part.
	message.Header.Set(
		jetstream.ScheduleHeader,
		"@at "+time.Now().Add(delay).UTC().Format(time.RFC3339Nano),
	)
	message.Header.Set(jetstream.ScheduleTargetHeader, target)
	message.Header.Set(jetstream.ScheduleTTLHeader, ttl.String())

	_, err = r.js.PublishMsg(ctx, message)

	return err
}

func (r *NATS) consumeJetStream(
	handler contractqueue.Handler,
	key string,
	subjects []string,
	options queueOptions,
) error {
	if options.queue == "" {
		return errors.New("nats: queue is required for JetStream consumers")
	}

	consumeContexts := make([]jetstream.ConsumeContext, 0, len(subjects))
	defer func() {
		for _, consumeContext := range consumeContexts {
			consumeContext.Stop()
		}
	}()

	for _, subject := range subjects {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		streamName, _, err := r.ensureStream(ctx, subject, 0, options.needsSchedules(), options.ttl, options)
		if err != nil {
			cancel()
			return err
		}

		durable := consumerName(options.queue, subject)
		consumer, err := r.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
			Durable:       durable,
			AckPolicy:     jetstream.AckExplicitPolicy,
			FilterSubject: subject,
			AckWait:       options.ackWait,
			MaxAckPending: options.maxAckPending,
			MaxDeliver:    -1,
		})
		cancel()
		if err != nil {
			return err
		}

		color.Infof(
			"[queue.nats] consumer %s subscribed to %s via JetStream stream %s (durable %s, ack_wait %s, max_ack_pending %d)\n",
			key,
			subject,
			streamName,
			durable,
			options.ackWait,
			options.maxAckPending,
		)

		consumeContext, err := consumer.Consume(func(message jetstream.Msg) {
			stopKeepAlive := keepAlive(message, options.ackWait)
			defer stopKeepAlive()

			headers := headersFromNATS(message.Headers())
			retried := contractqueue.RetryCount(headers)
			response, consumeErr := contractqueue.CallHandler("nats", handler, message.Data(), options.handlerTimeout)
			if consumeErr == nil {
				if ackErr := acknowledge(message, response); ackErr != nil {
					color.Errorf("[queue.nats] acknowledge JetStream message: %v\n", ackErr)
				}
				return
			}

			if _, ok := contractqueue.RetryAfter(options.retry, retried); ok {
				if retryErr := r.requeue(message.Data(), headers, message.Subject(), options, retried); retryErr != nil {
					color.Errorf("[queue.nats] requeue JetStream message: %v\n", retryErr)
					if nakErr := message.Nak(); nakErr != nil {
						color.Errorf("[queue.nats] retry JetStream message: %v\n", nakErr)
					}
					return
				}

				if ackErr := acknowledge(message, response); ackErr != nil {
					color.Errorf("[queue.nats] acknowledge requeued JetStream message: %v\n", ackErr)
				}
				return
			}

			if options.errorEnabled {
				if publishErr := r.publishError(
					message.Data(),
					options,
					message.Subject(),
					retried,
					consumeErr,
				); publishErr != nil {
					color.Errorf("[queue.nats] publish error message: %v\n", publishErr)
					_ = message.Nak()
					return
				}
			}

			if ackErr := acknowledge(message, response); ackErr != nil {
				color.Errorf("[queue.nats] acknowledge failed JetStream message: %v\n", ackErr)
			}
		})
		if err != nil {
			return err
		}
		consumeContexts = append(consumeContexts, consumeContext)
	}

	<-r.done
	return r.conn.LastError()
}

func (r *NATS) requeue(
	data []byte,
	headers contractqueue.Headers,
	subject string,
	options queueOptions,
	retried int,
) error {
	wait, ok := contractqueue.RetryAfter(options.retry, retried)
	if !ok {
		return fmt.Errorf("nats: retries exhausted")
	}

	// Tiered retry delay is independent of the original produce delay.
	options.delay = wait
	return r.publish(
		data,
		[]string{subject},
		options,
		contractqueue.WithRetryCount(headers, retried+1),
	)
}

func (r *NATS) ensureStream(
	ctx context.Context,
	target string,
	minimumAge time.Duration,
	scheduling bool,
	consumerTTL time.Duration,
	options queueOptions,
) (string, string, error) {
	r.streamMu.Lock()
	defer r.streamMu.Unlock()

	binding, cached := r.streams[target]
	consumerTTL = minimumTTL(binding.consumerTTL, consumerTTL)
	consumerTTLReady := consumerTTL <= 0 || (binding.consumerTTL > 0 && binding.consumerTTL <= consumerTTL)
	minimumAgeReady := binding.consumerTTL > 0 || binding.maxAge == 0 || binding.maxAge >= minimumAge
	if cached && consumerTTLReady && minimumAgeReady && (!scheduling || binding.schedule != "") {
		return binding.stream, binding.schedule, nil
	}

	streamName, err := r.js.StreamNameBySubject(ctx, target)
	targetExists := err == nil
	if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
		return "", "", fmt.Errorf("nats: find stream for subject %s: %w", target, err)
	}
	if !targetExists {
		streamName = options.stream
		if streamName == "" {
			streamName = "FRAMEWORK_QUEUE"
		}
	}

	scheduleSubject := ""
	if scheduling || (cached && binding.schedule != "") {
		scheduleSubject = options.scheduleSubject(streamName)
	}
	if scheduling && scheduleSubject == target {
		return "", "", errors.New("nats: schedule subject must differ from target subject")
	}

	stream, err := r.js.Stream(ctx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		maxAge := options.streamMaxAge(minimumAge)
		metadata := map[string]string{managedStreamMetadata: "true"}
		if consumerTTL > 0 {
			maxAge = consumerTTL
			metadata[consumerTTLStreamMetadata] = consumerTTL.String()
		}
		subjects := []string{target}
		if scheduling {
			subjects = append(subjects, scheduleSubject)
		}
		stream, err = r.js.CreateStream(ctx, jetstream.StreamConfig{
			Name:              streamName,
			Subjects:          subjects,
			MaxAge:            maxAge,
			Storage:           jetstream.FileStorage,
			AllowMsgSchedules: scheduling,
			AllowMsgTTL:       true,
			Metadata:          metadata,
		})
		if err != nil {
			return "", "", fmt.Errorf("nats: create queue stream %s: %w", streamName, err)
		}
		if scheduling {
			if err = requireSchedules(ctx, stream, streamName); err != nil {
				return "", "", err
			}
		}

		r.streams[target] = streamBinding{
			stream:      streamName,
			schedule:    scheduleSubject,
			maxAge:      maxAge,
			consumerTTL: consumerTTL,
		}
		return streamName, scheduleSubject, nil
	}
	if err != nil {
		return "", "", fmt.Errorf("nats: load queue stream %s: %w", streamName, err)
	}

	info := stream.CachedInfo()
	if info == nil {
		return "", "", fmt.Errorf("nats: stream %s returned no configuration", streamName)
	}

	config := info.Config
	config.Subjects = append([]string(nil), config.Subjects...)
	config.Metadata = maps.Clone(config.Metadata)
	changed := false
	if !targetExists {
		config.Subjects = append(config.Subjects, target)
		changed = true
	}

	if scheduling {
		scheduleOwner, ownerErr := r.js.StreamNameBySubject(ctx, scheduleSubject)
		if ownerErr == nil && scheduleOwner != streamName {
			return "", "", fmt.Errorf("nats: schedule subject %s belongs to stream %s", scheduleSubject, scheduleOwner)
		}
		if errors.Is(ownerErr, jetstream.ErrStreamNotFound) {
			config.Subjects = append(config.Subjects, scheduleSubject)
			changed = true
		} else if ownerErr != nil {
			return "", "", fmt.Errorf("nats: find stream for schedule subject %s: %w", scheduleSubject, ownerErr)
		}

		if !config.AllowMsgSchedules {
			config.AllowMsgSchedules = true
			changed = true
		}
	}
	if !config.AllowMsgTTL {
		config.AllowMsgTTL = true
		changed = true
	}

	managed := config.Metadata[managedStreamMetadata] == "true"
	effectiveConsumerTTL := time.Duration(0)
	if managed {
		effectiveConsumerTTL = minimumTTL(streamConsumerTTL(config.Metadata), consumerTTL)
		if effectiveConsumerTTL > 0 {
			if config.Metadata == nil {
				config.Metadata = make(map[string]string)
			}
			if config.Metadata[consumerTTLStreamMetadata] != effectiveConsumerTTL.String() {
				config.Metadata[consumerTTLStreamMetadata] = effectiveConsumerTTL.String()
				changed = true
			}
			if config.MaxAge != effectiveConsumerTTL {
				config.MaxAge = effectiveConsumerTTL
				changed = true
			}
		} else {
			maxAge := options.streamMaxAge(minimumAge)
			if config.MaxAge > 0 && config.MaxAge < maxAge {
				config.MaxAge = maxAge
				changed = true
			}
		}
	} else if consumerTTL > 0 {
		if config.MaxAge <= 0 || config.MaxAge > consumerTTL {
			return "", "", fmt.Errorf(
				"nats: stream %s max age %s does not satisfy consumer TTL %s",
				streamName,
				config.MaxAge,
				consumerTTL,
			)
		}
		effectiveConsumerTTL = config.MaxAge
	} else if config.MaxAge > 0 && config.MaxAge < minimumAge {
		return "", "", fmt.Errorf(
			"nats: stream %s max age %s is shorter than required message lifetime %s",
			streamName,
			config.MaxAge,
			minimumAge,
		)
	}

	if changed {
		stream, err = r.js.UpdateStream(ctx, config)
		if err != nil {
			return "", "", fmt.Errorf("nats: configure queue stream %s: %w", streamName, err)
		}
		info = stream.CachedInfo()
	}
	if scheduling {
		if err = requireSchedules(ctx, stream, streamName); err != nil {
			return "", "", err
		}
	}

	maxAge := time.Duration(0)
	if info != nil {
		maxAge = info.Config.MaxAge
	}
	r.streams[target] = streamBinding{
		stream:      streamName,
		schedule:    scheduleSubject,
		maxAge:      maxAge,
		consumerTTL: effectiveConsumerTTL,
	}

	return streamName, scheduleSubject, nil
}

// needsJetStream reports whether the queue must be consumed through JetStream.
// Scheduled retries are only visible to JetStream consumers: the broker appends
// them to the stream instead of publishing to core subscribers.
func (r queueOptions) needsJetStream() bool {
	return r.delayed || r.delay > 0 || r.ttl > 0 || contractqueue.HasPositiveRetryDelay(r.retry)
}

// needsSchedules reports whether the queue needs broker-side message schedules,
// either for a delayed first produce or for tiered retries.
func (r queueOptions) needsSchedules() bool {
	return r.delay > 0 || contractqueue.HasPositiveRetryDelay(r.retry)
}

func (r queueOptions) scheduleSubject(stream string) string {
	return strings.Trim(r.schedulePrefix, ".") + "." + stream
}

func (r queueOptions) delayedMessageTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}

	return r.retention
}

func (r queueOptions) streamMaxAge(minimumAge time.Duration) time.Duration {
	if minimumAge > r.retention {
		return minimumAge
	}

	return r.retention
}

func streamConsumerTTL(metadata map[string]string) time.Duration {
	ttl, err := time.ParseDuration(metadata[consumerTTLStreamMetadata])
	if err != nil || ttl <= 0 {
		return 0
	}

	return ttl
}

func minimumTTL(current, requested time.Duration) time.Duration {
	if current <= 0 {
		return requested
	}
	if requested <= 0 || current < requested {
		return current
	}

	return requested
}

func consumerName(queue, subject string) string {
	sum := sha256.Sum256([]byte(queue + "\x00" + subject))
	return fmt.Sprintf("FWQ_%x", sum[:12])
}

// Close closes the NATS connection and unblocks consumers.
func (r *NATS) Close() error {
	if r.conn == nil {
		r.signalDone()
		return nil
	}
	if !r.conn.IsClosed() {
		r.conn.Close()
	}

	color.Infof("[queue.nats] connection %s closed\n", r.label())

	r.signalDone()
	return nil
}

func (r *NATS) publishError(
	data []byte,
	options queueOptions,
	route string,
	retried int,
	consumeErr error,
) error {
	body, err := json.Marshal(contractqueue.BasicError{
		Exchange: options.topic,
		Queue:    options.queue,
		Route:    route,
		Retry:    retried,
		Message:  string(data),
		Error:    consumeErr.Error(),
	})
	if err != nil {
		return err
	}

	message := natsgo.NewMsg(options.errorSubject)
	message.Data = body
	if err = r.conn.PublishMsg(message); err != nil {
		return err
	}

	return r.conn.Flush()
}

func (r *NATS) queueOptions(key string) (queueOptions, error) {
	config, err := queueconfig.Load(key)
	if err != nil {
		return queueOptions{}, err
	}
	if err = config.EnsureEnabled(); err != nil {
		return queueOptions{}, err
	}
	if r.name != "" && config.Connection != r.name {
		return queueOptions{}, fmt.Errorf(
			"queue %s uses connection %s, not %s",
			key,
			config.Connection,
			r.name,
		)
	}

	delay, err := config.Duration("delay", 0)
	if err != nil {
		return queueOptions{}, err
	}
	ttl, err := config.Duration("ttl", 0)
	if err != nil {
		return queueOptions{}, err
	}
	retention, err := config.Duration("retention", r.cfg.GetDuration("nats.retention"))
	if err != nil {
		return queueOptions{}, err
	}
	if retention <= 0 {
		retention = time.Hour
	}

	queueName := config.String("queue", key)
	routes := config.Strings("routes")
	if len(routes) == 0 {
		route := config.String("route", "")
		if route != "" {
			routes = []string{route}
		}
	}
	retry, err := config.Retry()
	if err != nil {
		return queueOptions{}, err
	}
	errorSubject := config.String("error", r.cfg.GetString("nats.error"))
	if errorSubject == "" {
		errorSubject = "basic_error"
	}
	schedulePrefix := config.String("schedule_prefix", r.cfg.GetString("nats.schedule_prefix"))
	if schedulePrefix == "" {
		schedulePrefix = "_framework.queue.schedule"
	}
	ackWait, err := config.Duration("ack_wait", r.cfg.GetDuration("nats.ack_wait"))
	if err != nil {
		return queueOptions{}, err
	}
	if ackWait <= 0 {
		ackWait = defaultAckWait
	}
	maxAckPending := config.Int("max_ack_pending", r.cfg.GetInt("nats.max_ack_pending"))
	if maxAckPending <= 0 {
		maxAckPending = defaultMaxAckPending
	}
	handlerTimeout := contractqueue.DefaultHandlerTimeout
	if r.cfg.IsSet("nats.handler_timeout") {
		handlerTimeout = r.cfg.GetDuration("nats.handler_timeout")
	}
	handlerTimeout, err = config.Duration("handler_timeout", handlerTimeout)
	if err != nil {
		return queueOptions{}, err
	}

	return queueOptions{
		topic:          config.String("topic", ""),
		queue:          queueName,
		routes:         routes,
		delay:          delay,
		ttl:            ttl,
		delayed:        delay > 0 || config.Bool("delayed", false),
		retry:          retry,
		headers:        config.Headers(),
		errorEnabled:   config.Bool("error_enable", true),
		errorSubject:   errorSubject,
		stream:         config.String("stream", r.cfg.GetString("nats.stream")),
		schedulePrefix: schedulePrefix,
		retention:      retention,
		ackWait:        ackWait,
		maxAckPending:  maxAckPending,
		handlerTimeout: handlerTimeout,
	}, nil
}

func (r *NATS) signalDone() {
	r.doneOnce.Do(func() {
		close(r.done)
	})
}

func publishSubjects(topic, queue string, routes []string) []string {
	if len(routes) == 0 {
		if topic = subject(topic, ""); topic != "" {
			return []string{topic}
		}

		if queue = subject("", queue); queue != "" {
			return []string{queue}
		}

		return nil
	}

	subjects := make([]string, 0, len(routes))
	for _, route := range routes {
		if value := subject(topic, route); value != "" {
			subjects = append(subjects, value)
		}
	}

	return subjects
}

func subject(topic, route string) string {
	topic = strings.Trim(topic, ".")
	route = strings.Trim(route, ".")

	if topic == "" {
		return route
	}
	if route == "" {
		return topic
	}

	return topic + "." + route
}

func setHeaders(message *natsgo.Msg, headers contractqueue.Headers) {
	for key, value := range headers {
		if value == nil {
			continue
		}

		if message.Header == nil {
			message.Header = natsgo.Header{}
		}

		switch values := value.(type) {
		case []string:
			for _, item := range values {
				message.Header.Add(key, item)
			}
		default:
			message.Header.Set(key, fmt.Sprint(value))
		}
	}
}

func headersFromNATS(source natsgo.Header) contractqueue.Headers {
	if len(source) == 0 {
		return nil
	}

	headers := make(contractqueue.Headers, len(source))
	for key, values := range source {
		if strings.HasPrefix(strings.ToLower(key), "nats-") || len(values) == 0 {
			continue
		}

		switch {
		case strings.EqualFold(key, contractqueue.RetryHeader):
			key = contractqueue.RetryHeader
		case strings.EqualFold(key, contractqueue.DelayHeader):
			key = contractqueue.DelayHeader
		}

		headers[key] = append([]string(nil), values...)
	}
	if len(headers) == 0 {
		return nil
	}

	return headers
}
