package identity_test

import (
	"net/netip"
	"strings"
	"testing"

	"bidora/backend/internal/identity/browser"
)

func resolver(t *testing.T) *browser.ClientIPResolver {
	t.Helper()
	r, err := browser.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("2001:db8:ffff::/48")})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestClientIPResolverIgnoresSpoofedHeaderFromUntrustedPeer(t *testing.T) {
	r := resolver(t)
	peer := netip.MustParseAddrPort("198.51.100.7:443")
	for _, header := range []string{"203.0.113.9", "malformed", ""} {
		got, err := r.Resolve(peer, header)
		if err != nil || got != peer.Addr() {
			t.Fatal("untrusted peer header influenced client", got, err)
		}
	}
	mapped := netip.MustParseAddrPort("[::ffff:198.51.100.7]:443")
	got, err := r.Resolve(mapped, "203.0.113.9")
	if err != nil || got != netip.MustParseAddr("198.51.100.7") {
		t.Fatal("mapped direct peer not canonicalized", got, err)
	}
}
func TestClientIPResolverWalksTrustedChainRightToLeft(t *testing.T) {
	r := resolver(t)
	peer := netip.MustParseAddrPort("10.0.0.1:443")
	cases := map[string]netip.Addr{"198.51.100.7": netip.MustParseAddr("198.51.100.7"), "203.0.113.99, 198.51.100.7, 10.2.0.4": netip.MustParseAddr("198.51.100.7"), "10.3.0.8, 10.2.0.4": netip.MustParseAddr("10.3.0.8"), "::ffff:198.51.100.7, 10.2.0.4": netip.MustParseAddr("198.51.100.7")}
	for header, want := range cases {
		got, err := r.Resolve(peer, header)
		if err != nil || got != want {
			t.Fatal("trusted chain resolved incorrectly", header, got, want, err)
		}
	}
}
func TestClientIPResolverFailsClosedForMalformedTrustedChain(t *testing.T) {
	r := resolver(t)
	peer := netip.MustParseAddrPort("10.0.0.1:443")
	headers := []string{"", "malformed", "198.51.100.7:1234", "198.51.100.7,", "198.51.100.7,,10.0.0.2", strings.Repeat("1", 2049), strings.Repeat("198.51.100.1,", 33)}
	for _, header := range headers {
		got, err := r.Resolve(peer, header)
		if err == nil || err.Error() != "client_ip_unavailable" || got.IsValid() {
			t.Fatal("malformed trusted chain accepted", header, got, err)
		}
	}
	if got, err := r.Resolve(netip.AddrPort{}, "198.51.100.7"); err == nil || got.IsValid() {
		t.Fatal("invalid peer accepted")
	}
	var zero *browser.ClientIPResolver
	if _, err := zero.Resolve(peer, "198.51.100.7"); err == nil {
		t.Fatal("nil resolver accepted")
	}
}
func TestClientIPResolverRequiresExplicitValidPrefixes(t *testing.T) {
	if r, err := browser.NewClientIPResolver(nil); err == nil || err.Error() != "trusted_proxy_config_invalid" || r != nil {
		t.Fatal("empty trusted proxy config accepted")
	}
	prefixes := make([]netip.Prefix, 33)
	for i := range prefixes {
		prefixes[i] = netip.MustParsePrefix("10.0.0.0/8")
	}
	if r, err := browser.NewClientIPResolver(prefixes); err == nil || r != nil {
		t.Fatal("unbounded trusted proxy config accepted")
	}
	mapped := netip.MustParsePrefix("::ffff:10.0.0.0/104")
	r, err := browser.NewClientIPResolver([]netip.Prefix{mapped})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Resolve(netip.MustParseAddrPort("10.1.1.1:443"), "198.51.100.7")
	if err != nil || got != netip.MustParseAddr("198.51.100.7") {
		t.Fatal("mapped proxy prefix not canonicalized", got, err)
	}
}
