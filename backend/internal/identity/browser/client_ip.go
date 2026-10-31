package browser

import (
	"errors"
	"net/netip"
	"strings"
)

const maxForwardedHeaderBytes = 2048
const maxForwardedHops = 32

type ClientIPResolver struct{ trusted []netip.Prefix }

func NewClientIPResolver(trusted []netip.Prefix) (*ClientIPResolver, error) {
	if len(trusted) == 0 || len(trusted) > maxForwardedHops {
		return nil, errors.New("trusted_proxy_config_invalid")
	}
	owned := make([]netip.Prefix, 0, len(trusted))
	for _, prefix := range trusted {
		if !prefix.IsValid() {
			return nil, errors.New("trusted_proxy_config_invalid")
		}
		if prefix.Addr().Is4In6() {
			bits := prefix.Bits() - 96
			if bits < 0 {
				return nil, errors.New("trusted_proxy_config_invalid")
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), bits)
		}
		owned = append(owned, prefix.Masked())
	}
	return &ClientIPResolver{owned}, nil
}

// Resolve ignores X-Forwarded-For from untrusted direct peers. A trusted peer
// must provide a complete valid chain; resolution strips trusted hops from the
// right and returns the first untrusted address.
func (r *ClientIPResolver) Resolve(peer netip.AddrPort, xForwardedFor string) (netip.Addr, error) {
	if r == nil || len(r.trusted) == 0 || !peer.IsValid() {
		return netip.Addr{}, errors.New("client_ip_unavailable")
	}
	direct := peer.Addr().Unmap()
	if !r.isTrusted(direct) {
		return direct, nil
	}
	if xForwardedFor == "" || len(xForwardedFor) > maxForwardedHeaderBytes {
		return netip.Addr{}, errors.New("client_ip_unavailable")
	}
	parts := strings.Split(xForwardedFor, ",")
	if len(parts) == 0 || len(parts) > maxForwardedHops {
		return netip.Addr{}, errors.New("client_ip_unavailable")
	}
	chain := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		address, err := netip.ParseAddr(value)
		if err != nil {
			return netip.Addr{}, errors.New("client_ip_unavailable")
		}
		chain = append(chain, address.Unmap())
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if !r.isTrusted(chain[i]) {
			return chain[i], nil
		}
	}
	return chain[0], nil
}
func (r *ClientIPResolver) isTrusted(address netip.Addr) bool {
	for _, prefix := range r.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
