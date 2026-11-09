package integration_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/broker"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

const (
	queueWait = 30 * time.Second
	queuePoll = 200 * time.Millisecond
)

func natsURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("NATS_URL")
	require.NotEmpty(t, url, "NATS_URL is not set")

	return url
}

func newPublisher(t *testing.T) *broker.Broker {
	t.Helper()

	b, err := broker.New(natsURL(t), broker.Settings{AckWait: time.Minute, ShutdownTimeout: time.Second},
		slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	return b
}

// The worker consumes the mail topic and delivers what is on it.
func TestWorkerConsumesTheMailTopic(t *testing.T) {
	name := sanitizeTestName(t)
	email := name + "@test.com"

	body, err := entity.EncodeEmail(entity.WelcomeEmail{To: email, Username: name, LoginURL: "http://localhost/login"})
	require.NoError(t, err)

	require.NoError(t, newPublisher(t).Publish(t.Context(), entity.EmailQueue, "it-"+name, body))

	require.Eventually(t, func() bool { return findMailTo(t, email) != "" },
		mailWait, mailPoll, "the email never reached Mailpit")
}

// A message the mail handler can never decode is not retried: it lands on the poison topic,
// with the reason, and is not lost.
func TestAnUndecodableMessageGoesToThePoisonTopic(t *testing.T) {
	id := "it-poison-" + sanitizeTestName(t)

	require.NoError(t, newPublisher(t).Publish(t.Context(), entity.EmailQueue, id, []byte(`"not an email"`)))

	nc, err := nats.Connect(natsURL(t))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		msg, err := js.GetLastMsg(broker.PoisonTopic, broker.PoisonTopic)
		if err != nil {
			return false
		}

		return msg.Header.Get(wmnats.WatermillUUIDHdr) == id && msg.Header.Get("reason_poisoned") != ""
	}, queueWait, queuePoll, "message %s never reached the poison topic", id)
}
