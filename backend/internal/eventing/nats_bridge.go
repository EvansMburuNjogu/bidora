package eventing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"os"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type NATSConfig struct {
	URL, CredentialsFile, ServerName string
	RootCAs                          *x509.CertPool
	ConnectTimeout                   time.Duration
}
type Bridge struct {
	connection   *nats.Conn
	js           nats.JetStreamContext
	subscription *nats.Subscription
	outboxes     []*Outbox
	consumer     ReservationConsumer
	ids          IDGenerator
	clock        Clock
	next         int
}

func NewBridge(config NATSConfig, outboxes []*Outbox, consumer ReservationConsumer, ids IDGenerator, clock Clock) (*Bridge, error) {
	if len(outboxes) == 0 || consumer == nil || ids == nil || clock == nil || config.CredentialsFile == "" || config.RootCAs == nil || config.ServerName == "" || config.ConnectTimeout < time.Second || config.ConnectTimeout > 30*time.Second {
		return nil, ErrInvalid
	}
	parsed, e := url.Parse(config.URL)
	if e != nil || parsed.Scheme != "tls" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return nil, ErrInvalid
	}
	info, e := os.Stat(config.CredentialsFile)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, ErrInvalid
	}
	connection, e := nats.Connect(config.URL, nats.UserCredentials(config.CredentialsFile), nats.Secure(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.ServerName, RootCAs: config.RootCAs}), nats.Timeout(config.ConnectTimeout), nats.NoEcho(), nats.Name("bidora-pilot-ai-entitlement"), nats.MaxReconnects(-1))
	if e != nil {
		return nil, ErrUnavailable
	}
	js, e := connection.JetStream(nats.PublishAsyncMaxPending(64))
	if e != nil {
		connection.Close()
		return nil, ErrUnavailable
	}
	stream, e := js.StreamInfo(ReservationStream)
	if e != nil || stream.Config.Name != ReservationStream || !contains(stream.Config.Subjects, ReservationSubject) || stream.Config.Storage != nats.FileStorage || stream.Config.Retention != nats.LimitsPolicy || stream.Config.Duplicates < OutboxLease {
		connection.Close()
		return nil, ErrUnavailable
	}
	ci, e := js.ConsumerInfo(ReservationStream, ReservationDurable)
	if e != nil || ci.Config.AckPolicy != nats.AckExplicitPolicy || ci.Config.MaxDeliver != 9 || ci.Config.FilterSubject != ReservationSubject || ci.Config.DeliverPolicy != nats.DeliverAllPolicy || ci.Config.ReplayPolicy != nats.ReplayInstantPolicy || ci.Config.AckWait < 90*time.Second || ci.Config.MaxAckPending < 1 || ci.Config.MaxAckPending > 64 {
		connection.Close()
		return nil, ErrUnavailable
	}
	subscription, e := js.PullSubscribe(ReservationSubject, ReservationDurable, nats.Bind(ReservationStream, ReservationDurable))
	if e != nil {
		connection.Close()
		return nil, ErrUnavailable
	}
	return &Bridge{connection: connection, js: js, subscription: subscription, outboxes: outboxes, consumer: consumer, ids: ids, clock: clock}, nil
}
func (b *Bridge) Close() error {
	if b == nil || b.connection == nil {
		return nil
	}
	if e := b.connection.Drain(); e != nil {
		b.connection.Close()
		return ErrUnavailable
	}
	b.connection.Close()
	return nil
}
func (b *Bridge) ProcessNext(ctx context.Context) (bool, error) {
	if b == nil || ctx == nil {
		return false, ErrInvalid
	}
	for visited := 0; visited < len(b.outboxes); visited++ {
		idx := (b.next + visited) % len(b.outboxes)
		lease, e := b.ids.NewID()
		if e != nil || lease.IsZero() {
			return false, ErrUnavailable
		}
		at := b.clock.Now().UTC()
		if at.IsZero() {
			return false, ErrUnavailable
		}
		claim, found, e := b.outboxes[idx].Claim(ctx, lease, at)
		if e != nil {
			return false, ErrUnavailable
		}
		if !found {
			continue
		}
		b.next = (idx + 1) % len(b.outboxes)
		msg := nats.NewMsg(ReservationSubject)
		msg.Data = claim.Data
		msg.Header.Set(nats.MsgIdHdr, uuid.UUID(claim.ID).String())
		msg.Header.Set(nats.ExpectedStreamHdr, ReservationStream)
		if _, e = b.js.PublishMsg(msg, nats.Context(ctx)); e != nil {
			if retryErr := b.outboxes[idx].Retry(ctx, claim, b.clock.Now().UTC()); retryErr != nil {
				return true, ErrUnavailable
			}
			return true, nil
		}
		if e = b.outboxes[idx].Published(ctx, claim, b.clock.Now().UTC()); e != nil {
			return true, nil
		}
		return true, nil
	}
	messages, e := b.subscription.Fetch(1, nats.MaxWait(20*time.Millisecond))
	if errors.Is(e, nats.ErrTimeout) {
		return false, nil
	}
	if e != nil {
		return false, ErrUnavailable
	}
	if len(messages) != 1 {
		return false, ErrUnavailable
	}
	message := messages[0]
	event, e := DecodeReservation(message.Data)
	if e != nil {
		_ = message.Term()
		return true, nil
	}
	_, e = b.consumer.Consume(ctx, event)
	if e == nil {
		if ackErr := message.Ack(); ackErr != nil {
			return true, ErrUnavailable
		}
		return true, nil
	}
	if errors.Is(e, commercialapp.ErrAIReservationEventInvalid) || errors.Is(e, commercialapp.ErrAIReservationEventConflict) {
		_ = message.Term()
		return true, nil
	}
	metadata, _ := message.Metadata()
	delays := [...]time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}
	delivery := uint64(1)
	if metadata != nil && metadata.NumDelivered > 0 {
		delivery = metadata.NumDelivered
	}
	index := int(delivery - 1)
	if index >= len(delays) {
		index = len(delays) - 1
	}
	delay := delays[index]
	if nakErr := message.NakWithDelay(delay); nakErr != nil {
		return true, ErrUnavailable
	}
	return true, nil
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
