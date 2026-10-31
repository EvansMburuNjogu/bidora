package postgres_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/postgres"
)

func certificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: cert}, roots
}

func config(t *testing.T) postgres.Config {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "PG") {
			t.Setenv(key, "")
		}
	}
	_, roots := certificate(t)
	return postgres.Config{Host: "127.0.0.1", Port: 1, Database: "synthetic_db", User: "synthetic_runtime", Password: "secret-canary-password", RootCAs: roots, MaxConnections: 2, ConnectTimeout: 500 * time.Millisecond, OperationTimeout: time.Second, LockTimeout: 100 * time.Millisecond, MaxLifetime: time.Minute, MaxIdleTime: 10 * time.Second, HealthCheckPeriod: time.Second}
}

func closePool(t *testing.T, p *postgres.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Error(err)
	}
}

func TestConfigRejectsUnsafeDefaults(t *testing.T) {
	for _, mutate := range []func(*postgres.Config){
		func(c *postgres.Config) { c.Host = "" }, func(c *postgres.Config) { c.Host = "/tmp" }, func(c *postgres.Config) { c.Host = "a,b" }, func(c *postgres.Config) { c.Host = "postgres://secret-canary@host/db" }, func(c *postgres.Config) { c.Host = "[::1]" }, func(c *postgres.Config) { c.Host = "0.0.0.0" }, func(c *postgres.Config) { c.Host = "bad host" }, func(c *postgres.Config) { c.Port = 0 },
		func(c *postgres.Config) { c.Database = "" }, func(c *postgres.Config) { c.User = "a;DROP ROLE x" }, func(c *postgres.Config) { c.Password = "" }, func(c *postgres.Config) { c.Password = "secret-canary\x00" }, func(c *postgres.Config) { c.RootCAs = nil }, func(c *postgres.Config) { c.RootCAs = x509.NewCertPool() }, func(c *postgres.Config) { c.MaxConnections = 0 }, func(c *postgres.Config) { c.MaxConnections = 33 }, func(c *postgres.Config) { c.ConnectTimeout = 0 }, func(c *postgres.Config) { c.OperationTimeout = 31 * time.Second }, func(c *postgres.Config) { c.LockTimeout = 2 * time.Second }, func(c *postgres.Config) { c.MaxLifetime = 0 }, func(c *postgres.Config) { c.MaxIdleTime = 2 * time.Hour }, func(c *postgres.Config) { c.HealthCheckPeriod = 0 },
	} {
		c := config(t)
		mutate(&c)
		p, err := postgres.New(c)
		if err == nil || p != nil || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsafe configuration accepted or echoed")
		}
	}
	for _, host := range []string{"postgres.internal.example", "::1", "192.0.2.1"} {
		c := config(t)
		c.Host = host
		if err := c.Validate(); err != nil {
			t.Fatal("valid single host rejected")
		}
	}
}

func TestConfigRedactionAndAmbientSettings(t *testing.T) {
	c := config(t)
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if strings.Contains(fmt.Sprintf(format, c), "secret-canary") {
			t.Fatal("config formatting leaked")
		}
	}
	data, err := json.Marshal(c)
	if err != nil || strings.Contains(string(data), "secret-canary") {
		t.Fatal("config JSON leaked")
	}
	for _, key := range []string{"PGSERVICE", "PGSERVICEFILE", "PGPASSWORD", "PGHOST", "PGOPTIONS", "PGSSLMODE"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "secret-canary")
			p, err := postgres.New(c)
			if err == nil || p != nil || err.Error() != "ambient PG settings are not permitted by the explicit postgres adapter" {
				t.Fatal("ambient driver settings were not rejected before driver parsing")
			}
		})
	}
}

func TestTransactionRejectsInvalidLifecycleWithoutConnecting(t *testing.T) {
	p, err := postgres.New(config(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WithinTransaction(nil, func(context.Context, postgres.DBTX) error { return nil }); err == nil {
		t.Fatal("nil context accepted")
	}
	if err := p.WithinTransaction(context.Background(), nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	if err := p.WithinReadOnlyTransaction(context.Background(), nil); err == nil {
		t.Fatal("nil read-only callback accepted")
	}
	closePool(t, p)
	called := false
	if err := p.WithinTransaction(context.Background(), func(context.Context, postgres.DBTX) error { called = true; return nil }); err == nil || called {
		t.Fatal("closed pool began transaction")
	}
	var zero postgres.Pool
	if err := zero.WithinTransaction(context.Background(), func(context.Context, postgres.DBTX) error { return nil }); err == nil {
		t.Fatal("zero pool began transaction")
	}
}

func TestLazyPoolCancellationAndClose(t *testing.T) {
	p, err := postgres.New(config(t))
	if err != nil {
		t.Fatal(err)
	}
	defer closePool(t, p)
	if err := p.Ping(nil); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("caller cancellation lost")
	}
	if err := p.Close(nil); err == nil {
		t.Fatal("nil close context accepted")
	}
	closePool(t, p)
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("closed pool accepted work")
	}
	var zero postgres.Pool
	if err := zero.Ping(context.Background()); err == nil {
		t.Fatal("zero pool accepted work")
	}
	if err := zero.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCloseCancelsBlockedWork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	c := config(t)
	c.Port = uint16(listener.Addr().(*net.TCPAddr).Port)
	p, err := postgres.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer closePool(t, p)
	result := make(chan error, 1)
	go func() { result <- p.Ping(context.Background()) }()
	select {
	case conn := <-accepted:
		defer conn.Close() // Never answer the SSL request.
	case <-time.After(2 * time.Second):
		t.Fatal("connection was not attempted")
	}
	closePool(t, p)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("blocked peer reported ready")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not cancel active work")
	}
}

// This synthetic wire peer tests rejection boundaries, not PostgreSQL success.
func peer(t *testing.T, cert tls.Certificate, cleartext bool) (uint16, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		var request [8]byte
		if _, err = io.ReadFull(conn, request[:]); err != nil {
			result <- err
			return
		}
		if binary.BigEndian.Uint32(request[:4]) != 8 || binary.BigEndian.Uint32(request[4:]) != 80877103 {
			result <- errors.New("client did not request PostgreSQL TLS")
			return
		}
		if _, err = conn.Write([]byte{'S'}); err != nil {
			result <- err
			return
		}
		secure := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		if err = secure.Handshake(); err != nil {
			if cleartext {
				result <- err
			} else {
				result <- nil
			}
			return
		}
		if !cleartext {
			result <- errors.New("untrusted certificate accepted")
			return
		}
		var header [4]byte
		if _, err = io.ReadFull(secure, header[:]); err != nil {
			result <- err
			return
		}
		size := int(binary.BigEndian.Uint32(header[:])) - 4
		if size < 4 || size > 4096 {
			result <- errors.New("unexpected startup size")
			return
		}
		body := make([]byte, size)
		if _, err = io.ReadFull(secure, body); err != nil {
			result <- err
			return
		}
		if !strings.Contains(string(body), "synthetic_db\x00") || !strings.Contains(string(body), "synthetic_runtime\x00") || strings.Contains(string(body), "secret-canary") {
			result <- errors.New("explicit startup configuration incorrect")
			return
		}
		if _, err = secure.Write([]byte{'R', 0, 0, 0, 8, 0, 0, 0, 3}); err != nil {
			result <- err
			return
		}
		var response [256]byte
		n, readErr := secure.Read(response[:])
		if n != 0 || readErr == nil {
			result <- errors.New("client answered forbidden cleartext authentication")
			return
		}
		if timeout, ok := readErr.(net.Error); ok && timeout.Timeout() {
			result <- errors.New("client did not reject authentication promptly")
			return
		}
		result <- nil
	}()
	return uint16(listener.Addr().(*net.TCPAddr).Port), result
}

func TestTLSVerificationAndTrustRootsAreCopied(t *testing.T) {
	cert, _ := certificate(t)
	port, done := peer(t, cert, false)
	c := config(t)
	c.Port = port
	p, err := postgres.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer closePool(t, p)
	c.RootCAs.AddCert(cert.Leaf) // Must not alter the already constructed pool.
	err = p.Ping(context.Background())
	var verification *tls.CertificateVerificationError
	if err == nil || !errors.As(err, &verification) {
		t.Fatal("expected a TLS verification failure")
	}
	if strings.Contains(fmt.Sprintf("%#v", err), "synthetic_db") || strings.Contains(err.Error(), "secret-canary") {
		t.Fatal("connection error leaked private configuration")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("peer did not finish")
	}
}

func TestSCRAMRequiredOverVerifiedTLS(t *testing.T) {
	cert, roots := certificate(t)
	port, done := peer(t, cert, true)
	c := config(t)
	c.Port = port
	c.RootCAs = roots
	p, err := postgres.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer closePool(t, p)
	if err = p.Ping(context.Background()); err == nil {
		t.Fatal("cleartext authentication accepted")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("peer did not finish")
	}
}
