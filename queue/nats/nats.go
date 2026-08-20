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

	return driver, nil
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
	if options.delayed || options.ttl > 0 {
		return r.consumeJetStream(handler, subjects, options)
	}

	callback := func(message *natsgo.Msg) {
		headers := headersFromNATS(message.Header)
		retried := contractqueue.RetryCount(headers)
		_, consumeErr := handler(message.Data)
		if consumeErr == nil {
			return
		}
		if _, ok := contractqueue.RetryAfter(options.retry, retried); ok {
			if retryErr := r.requeue(message.Data, headers, message.Subject, options, retried); retryErr == nil {
				return
			} else {
				color.Errorf("[queue.nats] requeue message: %v", retryErr)
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
				color.Errorf("[queue.nats] publish error message: %v", publishErr)
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

	_, err = r.js.PublishMsg(
		ctx,
		message,
		jetstream.WithScheduleAt(time.Now().Add(delay)),
		jetstream.WithScheduleTarget(target),
		jetstream.WithScheduleTTL(ttl),
	)

	return err
}

func (r *NATS) consumeJetStream(
	handler contractqueue.Handler,
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
		streamName, _, err := r.ensureStream(ctx, subject, 0, options.delayed, options.ttl, options)
		if err != nil {
			cancel()
			return err
		}

		consumer, err := r.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
			Durable:       consumerName(options.queue, subject),
			AckPolicy:     jetstream.AckExplicitPolicy,
			FilterSubject: subject,
			MaxDeliver:    -1,
		})
		cancel()
		if err != nil {
			return err
		}

		consumeContext, err := consumer.Consume(func(message jetstream.Msg) {
			headers := headersFromNATS(message.Headers())
			retried := contractqueue.RetryCount(headers)
			response, consumeErr := handler(message.Data())
			if consumeErr == nil {
				if ackErr := acknowledge(message, response); ackErr != nil {
					color.Errorf("[queue.nats] acknowledge JetStream message: %v", ackErr)
				}
				return
			}

			if _, ok := contractqueue.RetryAfter(options.retry, retried); ok {
				if retryErr := r.requeue(message.Data(), headers, message.Subject(), options, retried); retryErr != nil {
					color.Errorf("[queue.nats] requeue JetStream message: %v", retryErr)
					if nakErr := message.Nak(); nakErr != nil {
						color.Errorf("[queue.nats] retry JetStream message: %v", nakErr)
					}
					return
				}

				if ackErr := acknowledge(message, response); ackErr != nil {
					color.Errorf("[queue.nats] acknowledge requeued JetStream message: %v", ackErr)
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
					color.Errorf("[queue.nats] publish error message: %v", publishErr)
					_ = message.Nak()
					return
				}
			}

			if ackErr := acknowledge(message, response); ackErr != nil {
				color.Errorf("[queue.nats] acknowledge failed JetStream message: %v", ackErr)
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
		_, err = r.js.CreateStream(ctx, jetstream.StreamConfig{
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
	}, nil
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
