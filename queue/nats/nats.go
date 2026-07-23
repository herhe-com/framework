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
)

const (
	managedStreamMetadata     = "framework.queue.managed"
	consumerTTLStreamMetadata = "framework.queue.consumer_ttl"
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
	done     chan struct{}
	doneOnce sync.Once
	streamMu sync.Mutex
	streams  map[string]streamBinding
}

var _ contractqueue.Driver = (*NATS)(nil)

// NewNATS creates and connects a NATS queue driver.
func NewNATS(configs map[string]any) (*NATS, error) {
	cfg := viper.New()
	cfg.Set("nats", configs)

	driver := &NATS{
		cfg:     cfg,
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

// Producer publishes the message to every topic-and-route NATS subject.
func (r *NATS) Producer(data []byte, options contractqueue.ProducerOptions) error {
	if r.conn == nil || r.conn.IsClosed() {
		return natsgo.ErrConnectionClosed
	}

	subjects := publishSubjects(options.Topic, options.Queue, options.Routes)
	if len(subjects) == 0 {
		return errors.New("nats: subject is required")
	}
	if options.Delay > 0 {
		for _, subject := range subjects {
			if err := r.publishDelayed(
				data,
				subject,
				options.Delay,
				options.TTL,
				options.Headers,
			); err != nil {
				return err
			}
		}

		return nil
	}
	if options.TTL > 0 {
		for _, subject := range subjects {
			if err := r.publishWithTTL(data, subject, options.TTL, options.Headers); err != nil {
				return err
			}
		}

		return nil
	}

	for _, subject := range subjects {
		message := natsgo.NewMsg(subject)
		message.Data = data
		setHeaders(message, options.Headers)

		if err := r.conn.PublishMsg(message); err != nil {
			return err
		}
	}

	return r.conn.Flush()
}

// Consumer consumes through core NATS or a durable JetStream consumer and blocks until Close is called.
func (r *NATS) Consumer(handler contractqueue.Handler, options contractqueue.ConsumerOptions) error {
	if handler == nil {
		return errors.New("nats: handler is required")
	}
	if r.conn == nil || r.conn.IsClosed() {
		return natsgo.ErrConnectionClosed
	}
	if options.Retry < 0 {
		options.Retry = 0
	}

	subject := subject(options.Topic, options.Route)
	if subject == "" {
		return errors.New("nats: subject is required")
	}
	if options.Delayed || options.TTL > 0 {
		return r.consumeJetStream(handler, subject, options)
	}

	callback := func(message *natsgo.Msg) {
		headers := headersFromNATS(message.Header)
		retried := contractqueue.RetryCount(headers)
		consumeErr := handler(message.Data)
		if consumeErr == nil {
			return
		}
		if retried < options.Retry {
			if retryErr := r.requeue(message.Data, headers, options, retried); retryErr == nil {
				return
			} else {
				color.Errorf("[queue.nats] requeue message: %v", retryErr)
			}
		}

		if publishErr := r.publishError(
			message.Data,
			options.Topic,
			options.Queue,
			options.Route,
			retried,
			consumeErr,
		); publishErr != nil {
			color.Errorf("[queue.nats] publish error message: %v", publishErr)
		}
	}

	var err error
	if options.Queue == "" {
		_, err = r.conn.Subscribe(subject, callback)
	} else {
		_, err = r.conn.QueueSubscribe(subject, options.Queue, callback)
	}
	if err != nil {
		return err
	}

	if err = r.conn.Flush(); err != nil {
		return err
	}

	<-r.done
	return r.conn.LastError()
}

func (r *NATS) publishWithTTL(data []byte, target string, ttl time.Duration, headers contractqueue.Headers) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, _, err := r.ensureStream(ctx, target, ttl, false, 0); err != nil {
		return err
	}

	message := natsgo.NewMsg(target)
	message.Data = data
	setHeaders(message, headers)

	_, err := r.js.PublishMsg(ctx, message, jetstream.WithMsgTTL(ttl))
	return err
}

func (r *NATS) publishDelayed(data []byte, target string, delay, ttl time.Duration, headers contractqueue.Headers) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ttl = r.delayedMessageTTL(ttl)
	_, scheduleSubject, err := r.ensureStream(ctx, target, delay+ttl, true, 0)
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
	subject string,
	options contractqueue.ConsumerOptions,
) error {
	if options.Queue == "" {
		return errors.New("nats: queue is required for JetStream consumers")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	streamName, _, err := r.ensureStream(ctx, subject, 0, options.Delayed, options.TTL)
	if err != nil {
		return err
	}

	consumer, err := r.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       consumerName(options.Queue, subject),
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		MaxDeliver:    -1,
	})
	if err != nil {
		return err
	}

	consumeContext, err := consumer.Consume(func(message jetstream.Msg) {
		headers := headersFromNATS(message.Headers())
		retried := contractqueue.RetryCount(headers)
		consumeErr := handler(message.Data())
		if consumeErr == nil {
			if ackErr := message.Ack(); ackErr != nil {
				color.Errorf("[queue.nats] acknowledge JetStream message: %v", ackErr)
			}
			return
		}

		if retried < options.Retry {
			if retryErr := r.requeue(message.Data(), headers, options, retried); retryErr != nil {
				color.Errorf("[queue.nats] requeue JetStream message: %v", retryErr)
				if nakErr := message.Nak(); nakErr != nil {
					color.Errorf("[queue.nats] retry JetStream message: %v", nakErr)
				}
				return
			}

			if ackErr := message.Ack(); ackErr != nil {
				color.Errorf("[queue.nats] acknowledge requeued JetStream message: %v", ackErr)
			}
			return
		}

		if publishErr := r.publishError(
			message.Data(),
			options.Topic,
			options.Queue,
			options.Route,
			retried,
			consumeErr,
		); publishErr != nil {
			color.Errorf("[queue.nats] publish error message: %v", publishErr)
			_ = message.Nak()
			return
		}

		if ackErr := message.Ack(); ackErr != nil {
			color.Errorf("[queue.nats] acknowledge failed JetStream message: %v", ackErr)
		}
	})
	if err != nil {
		return err
	}
	defer consumeContext.Stop()

	<-r.done
	return r.conn.LastError()
}

func (r *NATS) requeue(
	data []byte,
	headers contractqueue.Headers,
	options contractqueue.ConsumerOptions,
	retried int,
) error {
	delay := time.Duration(0)
	if options.Delayed {
		delay = contractqueue.RetryDelay(headers)
	}

	return r.Producer(data, contractqueue.ProducerOptions{
		Topic:   options.Topic,
		Queue:   options.Queue,
		Routes:  []string{options.Route},
		Delay:   delay,
		TTL:     options.TTL,
		Headers: contractqueue.WithRetryCount(headers, retried+1),
	})
}

func (r *NATS) ensureStream(
	ctx context.Context,
	target string,
	minimumAge time.Duration,
	scheduling bool,
	consumerTTL time.Duration,
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
		streamName = r.cfg.GetString("nats.stream")
		if streamName == "" {
			streamName = "FRAMEWORK_QUEUE"
		}
	}

	scheduleSubject := ""
	if scheduling || (cached && binding.schedule != "") {
		scheduleSubject = r.scheduleSubject(streamName)
	}
	if scheduling && scheduleSubject == target {
		return "", "", errors.New("nats: schedule subject must differ from target subject")
	}

	stream, err := r.js.Stream(ctx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		maxAge := r.streamMaxAge(minimumAge)
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
			maxAge := r.streamMaxAge(minimumAge)
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

func (r *NATS) scheduleSubject(stream string) string {
	prefix := r.cfg.GetString("nats.schedule_prefix")
	if prefix == "" {
		prefix = "_framework.queue.schedule"
	}

	return strings.Trim(prefix, ".") + "." + stream
}

func (r *NATS) retention() time.Duration {
	retention := r.cfg.GetDuration("nats.retention")
	if retention <= 0 {
		retention = time.Hour
	}

	return retention
}

func (r *NATS) delayedMessageTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}

	return r.retention()
}

func (r *NATS) streamMaxAge(minimumAge time.Duration) time.Duration {
	retention := r.retention()
	if minimumAge > retention {
		return minimumAge
	}

	return retention
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

func (r *NATS) publishError(data []byte, topic, queue, route string, retried int, consumeErr error) error {
	errorSubject := r.cfg.GetString("nats.error")
	if errorSubject == "" {
		errorSubject = "basic_error"
	}

	body, err := json.Marshal(contractqueue.BasicError{
		Exchange: topic,
		Queue:    queue,
		Route:    route,
		Retry:    retried,
		Message:  string(data),
		Error:    consumeErr.Error(),
	})
	if err != nil {
		return err
	}

	message := natsgo.NewMsg(errorSubject)
	message.Data = body
	if err = r.conn.PublishMsg(message); err != nil {
		return err
	}

	return r.conn.Flush()
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
