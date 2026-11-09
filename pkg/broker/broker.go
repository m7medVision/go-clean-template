package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/m7medvision/go-clean-template/pkg/sentrylog"
	"github.com/nats-io/nats.go"
)

const (
	PoisonTopic = "poison"

	_group      = "worker"
	_multiplier = 2
)

var errPermanent = errors.New("permanent")

type Settings struct {
	MaxRetries      int
	RetryInitial    time.Duration
	RetryMax        time.Duration
	AckWait         time.Duration
	ShutdownTimeout time.Duration
}

type HandlerFunc func(ctx context.Context, body []byte) error

type Broker struct {
	pub    message.Publisher
	sub    message.Subscriber
	router *message.Router
	l      *slog.Logger
}

func Permanent(err error) error {
	return fmt.Errorf("%w: %w", errPermanent, err)
}

func IsPermanent(err error) bool {
	return errors.Is(err, errPermanent)
}

func New(url string, s Settings, l *slog.Logger) (*Broker, error) {
	logger := watermill.NewSlogLoggerWithLevelMapping(l, map[slog.Level]slog.Level{slog.LevelInfo: slog.LevelDebug})
	js := wmnats.JetStreamConfig{
		AutoProvision:     true,
		TrackMsgID:        true,
		DurablePrefix:     _group,
		DurableCalculator: name,
	}
	opts := []nats.Option{nats.RetryOnFailedConnect(false)}

	pub, err := wmnats.NewPublisher(wmnats.PublisherConfig{URL: url, NatsOptions: opts, JetStream: js}, logger)
	if err != nil {
		return nil, fmt.Errorf("broker - New - publisher: %w", err)
	}

	sub, err := wmnats.NewSubscriber(wmnats.SubscriberConfig{
		URL:               url,
		NatsOptions:       opts,
		QueueGroupPrefix:  _group,
		SubjectCalculator: subjects,
		AckWaitTimeout:    s.AckWait,
		CloseTimeout:      s.ShutdownTimeout,
		JetStream:         js,
	}, logger)
	if err != nil {
		_ = pub.Close()

		return nil, fmt.Errorf("broker - New - subscriber: %w", err)
	}

	router, err := newRouter(pub, s, logger, l)
	if err != nil {
		_ = pub.Close()
		_ = sub.Close()

		return nil, err
	}

	return &Broker{pub: pub, sub: sub, router: router, l: l}, nil
}

func newRouter(pub message.Publisher, s Settings, logger watermill.LoggerAdapter, l *slog.Logger) (*message.Router, error) {
	router, err := message.NewRouter(message.RouterConfig{CloseTimeout: s.ShutdownTimeout}, logger)
	if err != nil {
		return nil, fmt.Errorf("broker - New - router: %w", err)
	}

	//nolint:godox // the owner asked for this TODO; the decision is tracked in the spec.
	// TODO: decide how poison messages are inspected, replayed or discarded; until then they only
	// land on PoisonTopic and are logged. See docs/specs/nats-queue.md, section TODO.
	poison, err := middleware.PoisonQueue(pub, PoisonTopic)
	if err != nil {
		return nil, fmt.Errorf("broker - New - poison queue: %w", err)
	}

	retry := middleware.Retry{
		MaxRetries:      s.MaxRetries,
		InitialInterval: s.RetryInitial,
		MaxInterval:     s.RetryMax,
		Multiplier:      _multiplier,
		ShouldRetry:     func(p middleware.RetryParams) bool { return !IsPermanent(p.Err) },
		Logger:          logger,
	}

	router.AddMiddleware(sentrylog.Watermill, poison, giveUp(l), retry.Middleware, middleware.Recoverer)

	return router, nil
}

func (b *Broker) Publish(ctx context.Context, topic, id string, body []byte) error {
	msg := message.NewMessage(id, body)
	msg.SetContext(ctx)

	if err := b.pub.Publish(topic, msg); err != nil {
		return fmt.Errorf("broker - Publish %s: %w", topic, err)
	}

	return nil
}

func (b *Broker) Handle(topic string, h HandlerFunc) {
	b.router.AddConsumerHandler(topic, topic, b.sub, func(msg *message.Message) error {
		return h(msg.Context(), msg.Payload)
	})
}

func (b *Broker) Run(ctx context.Context) error {
	if err := b.router.Run(ctx); err != nil {
		return fmt.Errorf("broker - Run: %w", err)
	}

	return nil
}

func (b *Broker) Close() error {
	return errors.Join(b.router.Close(), b.sub.Close(), b.pub.Close())
}

func giveUp(l *slog.Logger) message.HandlerMiddleware {
	return func(h message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			out, err := h(msg)
			if err != nil {
				l.ErrorContext(msg.Context(), "broker - gave up on a message",
					"topic", message.SubscribeTopicFromCtx(msg.Context()), "message", msg.UUID, "error", err)
			}

			return out, err
		}
	}
}

func name(prefix, topic string) string {
	return prefix + "-" + topic
}

func subjects(prefix, topic string) *wmnats.SubjectDetail {
	return &wmnats.SubjectDetail{Primary: topic, QueueGroup: name(prefix, topic)}
}
