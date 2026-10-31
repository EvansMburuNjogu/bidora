package identity_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/googleoidc"
)

func googleTransportConfig() googleoidc.TransportConfig {
	return googleoidc.TransportConfig{RootCAs: x509.NewCertPool(), DialTimeout: time.Second, TLSHandshakeTimeout: time.Second, ResponseHeaderTimeout: time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 4}
}
func TestGoogleTransportRejectsEveryNonAllowlistedRequest(t *testing.T) {
	transport, err := googleoidc.NewTransport(googleTransportConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	requests := []*http.Request{}
	for _, spec := range []struct {
		method, target string
		body           bool
	}{{http.MethodGet, "https://evil.example/certs", false}, {http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs?x=1", false}, {http.MethodPost, "https://oauth2.googleapis.com/token?x=1", true}, {http.MethodGet, "https://oauth2.googleapis.com/token", false}, {http.MethodPost, "https://www.googleapis.com/oauth2/v3/certs", true}} {
		request, _ := http.NewRequest(spec.method, spec.target, nil)
		if spec.body {
			request.Body = http.NoBody
		}
		requests = append(requests, request)
	}
	for _, request := range requests {
		if response, err := transport.RoundTrip(request); err == nil || err.Error() != "google_oidc_transport_request_invalid" || response != nil {
			t.Fatal("unsafe Google request accepted", request.Method, request.URL, err)
		}
	}
}
func TestGoogleTransportAllowsOnlyFixedRequestsIntoNetworkLayer(t *testing.T) {
	transport, err := googleoidc.NewTransport(googleTransportConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	for _, spec := range []struct {
		method, target string
		body           bool
	}{{http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs", false}, {http.MethodPost, "https://oauth2.googleapis.com/token", true}} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		request, _ := http.NewRequestWithContext(ctx, spec.method, spec.target, nil)
		if spec.body {
			request.Body = http.NoBody
		}
		response, err := transport.RoundTrip(request)
		if err == nil || err.Error() != "google_oidc_transport_unavailable" || response != nil || strings.Contains(fmt.Sprintf("%#v", err), "context canceled") {
			t.Fatal("fixed Google request did not fail safely after canceled context", err)
		}
	}
}
func TestGoogleTransportRequiresExplicitBoundedConfiguration(t *testing.T) {
	valid := googleTransportConfig()
	cases := []googleoidc.TransportConfig{{}, func() googleoidc.TransportConfig { v := valid; v.RootCAs = nil; return v }(), func() googleoidc.TransportConfig { v := valid; v.DialTimeout = 0; return v }(), func() googleoidc.TransportConfig { v := valid; v.TLSHandshakeTimeout = 11 * time.Second; return v }(), func() googleoidc.TransportConfig { v := valid; v.MaxConnsPerHost = 0; return v }(), func() googleoidc.TransportConfig {
		v := valid
		v.MaxIdleConnsPerHost = 5
		v.MaxConnsPerHost = 4
		return v
	}()}
	for _, config := range cases {
		if transport, err := googleoidc.NewTransport(config); err == nil || transport != nil {
			t.Fatal("unsafe Google transport config accepted")
		}
	}
	if formatted := fmt.Sprintf("%#v", valid); strings.Contains(formatted, "CertPool") {
		t.Fatal("Google transport config formatting exposed trust configuration")
	}
	var zero *googleoidc.Transport
	zero.CloseIdleConnections()
	request, _ := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs", nil)
	if _, err := zero.RoundTrip(request); err == nil {
		t.Fatal("zero Google transport accepted request")
	}
}
