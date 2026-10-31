package identity_test

import (
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bufio"
	"context"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestSMTPSenderAndReplyPolicyReachActualSyntheticTransport(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	cfg := smtpConfig(t, fixture)
	cfg.SenderName = "Bidora Équipe"
	cfg.ReplyTo, _ = email.Parse("support@example.test")
	cfg.MaxConcurrency = 2
	delivery, e := deliverysmtp.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	recipient, _ := email.Parse("recipient@example.test")
	if e = delivery.DeliverRendered(context.Background(), recipient, "Synthetic policy check", "Synthetic body", "<p>Synthetic body</p>"); e != nil {
		t.Fatal(e)
	}
	select {
	case raw := <-fixture.message:
		message, e := mail.ReadMessage(bufio.NewReader(strings.NewReader(raw)))
		if e != nil {
			t.Fatal(e)
		}
		from, e := mail.ParseAddress(message.Header.Get("From"))
		if e != nil || from.Name != cfg.SenderName || from.Address != "sender@example.test" || message.Header.Get("Reply-To") != "support@example.test" || strings.Contains(raw, "Bcc:") {
			t.Fatal("actual message did not use exact configured sender policy")
		}
	case <-time.After(time.Second):
		t.Fatal("synthetic message missing")
	}
	bad := cfg
	bad.SenderName = "Injected\r\nBcc: stolen@example.test"
	if _, e = deliverysmtp.New(bad); e == nil {
		t.Fatal("sender header injection accepted")
	}
	bad = cfg
	bad.MaxConcurrency = 101
	if _, e = deliverysmtp.New(bad); e == nil {
		t.Fatal("unbounded delivery concurrency")
	}
}
func TestSMTPConcurrencyIsProcessBoundedAcrossDeliveryInstances(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	release := make(chan struct{})
	go func() {
		connection, e := listener.Accept()
		if e != nil {
			return
		}
		defer connection.Close()
		close(accepted)
		<-release
	}()
	defer close(release)
	cfg := smtpConfig(t, fixture)
	cfg.Port = uint16(listener.Addr().(*net.TCPAddr).Port)
	cfg.MaxConcurrency = 1
	cfg.Timeout = 300 * time.Millisecond
	first, e := deliverysmtp.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	second, e := deliverysmtp.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	recipient, _ := email.Parse("recipient@example.test")
	done := make(chan error, 1)
	go func() {
		done <- first.DeliverRendered(context.Background(), recipient, "Synthetic", "Synthetic", "<p>Synthetic</p>")
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("first delivery not admitted")
	}
	started := time.Now()
	if e = second.DeliverRendered(context.Background(), recipient, "Synthetic", "Synthetic", "<p>Synthetic</p>"); e == nil || time.Since(started) > 200*time.Millisecond {
		t.Fatal("second instance bypassed configured process concurrency")
	}
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("ungreeted transport unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("delivery deadline unbounded")
	}
}
