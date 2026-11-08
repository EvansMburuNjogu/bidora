package identity_test

import (
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"
)

func TestSMTPCommandOverallAndCallerBudgetsInterruptAStalledServer(t *testing.T) {
	for _, scenario := range []string{"command", "overall", "caller"} {
		t.Run(scenario, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				<-stop
			}()
			roots := x509.NewCertPool()
			roots.AddCert(&x509.Certificate{Raw: []byte{1}, RawSubject: []byte{1}})
			sender, _ := email.Parse("sender@example.test")
			cfg := deliverysmtp.Config{Host: "127.0.0.1", Port: uint16(listener.Addr().(*net.TCPAddr).Port), Username: "synthetic", Password: "synthetic", Sender: sender, TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots}, ConnectTimeout: time.Second, CommandTimeout: time.Second, DeliveryTimeout: 2 * time.Second}
			ctx := context.Background()
			cancel := func() {}
			switch scenario {
			case "command":
				cfg.CommandTimeout = 100 * time.Millisecond
			case "overall":
				cfg.DeliveryTimeout = 100 * time.Millisecond
			case "caller":
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
			}
			defer cancel()
			adapter, err := deliverysmtp.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			began := time.Now()
			err = adapter.DeliverRendered(ctx, sender, "Test", "Text", "<p>Text</p>")
			if err == nil || time.Since(began) > 800*time.Millisecond {
				t.Fatal("stalled SMTP ignored applicable deadline", scenario, err)
			}
		})
	}
}
