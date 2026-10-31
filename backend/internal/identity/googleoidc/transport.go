package googleoidc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type TransportConfig struct {
	RootCAs                                                                  *x509.CertPool
	DialTimeout, TLSHandshakeTimeout, ResponseHeaderTimeout, IdleConnTimeout time.Duration
	MaxIdleConns, MaxIdleConnsPerHost, MaxConnsPerHost                       int
}

func (TransportConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC transport config]")
}

type Transport struct {
	base      *http.Transport
	closeOnce sync.Once
}

func NewTransport(config TransportConfig) (*Transport, error) {
	if config.RootCAs == nil || !boundedTransportTimeout(config.DialTimeout) || !boundedTransportTimeout(config.TLSHandshakeTimeout) || !boundedTransportTimeout(config.ResponseHeaderTimeout) || config.IdleConnTimeout < time.Second || config.IdleConnTimeout > 2*time.Minute || config.MaxIdleConns < 1 || config.MaxIdleConns > 100 || config.MaxIdleConnsPerHost < 1 || config.MaxIdleConnsPerHost > 20 || config.MaxConnsPerHost < 1 || config.MaxConnsPerHost > 20 || config.MaxIdleConnsPerHost > config.MaxConnsPerHost {
		return nil, errors.New("google_oidc_transport_config_invalid")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.RootCAs.Clone()}
	dialer := &net.Dialer{Timeout: config.DialTimeout, KeepAlive: 30 * time.Second}
	base := &http.Transport{Proxy: nil, DialContext: dialer.DialContext, ForceAttemptHTTP2: true, MaxIdleConns: config.MaxIdleConns, MaxIdleConnsPerHost: config.MaxIdleConnsPerHost, MaxConnsPerHost: config.MaxConnsPerHost, IdleConnTimeout: config.IdleConnTimeout, TLSHandshakeTimeout: config.TLSHandshakeTimeout, ResponseHeaderTimeout: config.ResponseHeaderTimeout, ExpectContinueTimeout: time.Second, TLSClientConfig: tlsConfig, DisableCompression: true}
	return &Transport{base: base}, nil
}
func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t == nil || t.base == nil {
		return nil, errors.New("google_oidc_transport_unavailable")
	}
	if err := validateGoogleRequest(request); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, &transportError{cause: err}
	}
	return response, nil
}
func (t *Transport) CloseIdleConnections() {
	if t == nil || t.base == nil {
		return
	}
	t.closeOnce.Do(func() { t.base.CloseIdleConnections() })
}
func validateGoogleRequest(request *http.Request) error {
	if request == nil || request.URL == nil || request.URL.User != nil || request.URL.Fragment != "" || request.URL.RawQuery != "" {
		return errors.New("google_oidc_transport_request_invalid")
	}
	target := request.URL.String()
	if request.Method == http.MethodGet && target == googleJWKSEndpoint && request.Body == nil {
		return nil
	}
	if request.Method == http.MethodPost && target == googleTokenEndpoint && request.Body != nil {
		return nil
	}
	return errors.New("google_oidc_transport_request_invalid")
}
func boundedTransportTimeout(value time.Duration) bool {
	return value >= 100*time.Millisecond && value <= 10*time.Second
}

type transportError struct{ cause error }

func (e *transportError) Error() string                 { return "google_oidc_transport_unavailable" }
func (e *transportError) Unwrap() error                 { return e.cause }
func (e *transportError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }

var _ http.RoundTripper = (*Transport)(nil)
