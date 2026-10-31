package localprofile

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
)

// PostgresConfig is a validated local configuration, not an established
// connection. It deliberately contains no credentials or database fallback.
type PostgresConfig struct {
	Resources Config
	Address   netip.AddrPort
}

// LoadPostgres loads explicit resource settings and a literal loopback endpoint.
// It does not perform DNS resolution, inspect libpq defaults or connect.
func LoadPostgres(getenv func(string) string) (PostgresConfig, error) {
	resources, err := Load(getenv)
	if err != nil {
		return PostgresConfig{}, err
	}
	address, err := parseAddress(getenv("BIDORA_PG_ADDR"))
	if err != nil {
		return PostgresConfig{}, err
	}
	return PostgresConfig{Resources: resources, Address: address}, nil
}

func parseAddress(raw string) (netip.AddrPort, error) {
	invalid := func() (netip.AddrPort, error) {
		return netip.AddrPort{}, errors.New("BIDORA_PG_ADDR must be a literal loopback IP and explicit decimal port from 1 to 65535")
	}
	// A bounded endpoint is sufficient for IPv4 or bracketed IPv6 plus a port.
	if len(raw) > 64 {
		return invalid()
	}
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return invalid()
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" || ip.Is4In6() || !ip.IsLoopback() {
		return invalid()
	}
	if len(port) == 0 || len(port) > 5 || port[0] < '1' || port[0] > '9' {
		return invalid()
	}
	for i := 1; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return invalid()
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return invalid()
	}
	return netip.AddrPortFrom(ip, uint16(n)), nil
}
