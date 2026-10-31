// Package postgres provides portable PostgreSQL infrastructure, not tenant
// routing, authorization, schema setup or an implicit API startup dependency.
package postgres

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config must come from trusted server configuration/registry resolution, never
// directly from HTTP tenant identifiers. All limits and trust roots are explicit.
// Formatting and JSON redact the entire value; Password remains sensitive.
type Config struct {
	Host              string
	Port              uint16
	Database          string
	User              string
	Password          string
	RootCAs           *x509.CertPool
	MaxConnections    int32
	ConnectTimeout    time.Duration
	OperationTimeout  time.Duration
	LockTimeout       time.Duration
	MaxLifetime       time.Duration
	MaxIdleTime       time.Duration
	HealthCheckPeriod time.Duration
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED postgres config]") }
func (Config) MarshalJSON() ([]byte, error)  { return []byte(`"[REDACTED postgres config]"`), nil }

func (c Config) Validate() error {
	if !validHost(c.Host) || c.Port == 0 {
		return errors.New("postgres requires an explicit single host and port")
	}
	if !identifier(c.Database) || !identifier(c.User) {
		return errors.New("postgres requires explicit database and role identifiers")
	}
	if len(c.Password) == 0 || len(c.Password) > 1024 || strings.ContainsRune(c.Password, 0) {
		return errors.New("postgres requires an explicit bounded password")
	}
	if c.RootCAs == nil || len(c.RootCAs.Subjects()) == 0 {
		return errors.New("postgres requires explicit TLS trust roots")
	}
	if c.MaxConnections < 1 || c.MaxConnections > 32 {
		return errors.New("postgres pool size must be between 1 and 32")
	}
	if c.ConnectTimeout < time.Millisecond || c.ConnectTimeout > 30*time.Second || c.OperationTimeout < time.Millisecond || c.OperationTimeout > 30*time.Second || c.LockTimeout < time.Millisecond || c.LockTimeout > c.OperationTimeout {
		return errors.New("postgres operation and connection timeouts must be explicitly bounded")
	}
	if c.MaxLifetime < time.Second || c.MaxLifetime > 24*time.Hour || c.MaxIdleTime < time.Second || c.MaxIdleTime > time.Hour || c.MaxIdleTime > c.MaxLifetime || c.HealthCheckPeriod < time.Second || c.HealthCheckPeriod > time.Minute {
		return errors.New("postgres pool lifecycle settings must be explicitly bounded")
	}
	return nil
}

func identifier(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}

func validHost(host string) bool {
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Zone() == "" && !ip.Unmap().IsUnspecified() && !ip.Unmap().IsMulticast()
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range []byte(label) {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func poolConfig(c Config) (*pgxpool.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	// Never mutate process-global environment. Reject ambient libpq settings;
	// all actual endpoint/auth/session values come from the supplied Config.
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "PG") && value != "" {
			return nil, errors.New("ambient PG settings are not permitted by the explicit postgres adapter")
		}
	}
	// pgx requires ParseConfig-created configs. Parse a constant inert template,
	// not a secret-bearing DSN. Explicit paths/mode prevent home-file reads or
	// service redirection even if another goroutine mutates PG* environment.
	devNull := strings.ReplaceAll(strings.ReplaceAll(os.DevNull, `\`, `\\`), `'`, `\'`)
	p, err := pgxpool.ParseConfig("host=127.0.0.1 port=1 user=unused dbname=unused password=unused sslmode=disable sslcert='' sslkey='' sslrootcert='' servicefile='" + devNull + "' passfile='" + devNull + "' target_session_attrs=any connect_timeout=1 require_auth=scram-sha-256 channel_binding=prefer min_protocol_version=3.0 max_protocol_version=3.0")
	if err != nil {
		return nil, safeError("postgres_config_invalid", err)
	}
	cc := p.ConnConfig
	cc.Host, cc.Port, cc.Database, cc.User, cc.Password = c.Host, c.Port, c.Database, c.User, c.Password
	cc.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: c.Host, RootCAs: c.RootCAs.Clone()}
	cc.Fallbacks = nil
	cc.MaxProtocolMessageBodyLen = 16 * 1024 * 1024
	cc.ConnectTimeout = c.ConnectTimeout
	cc.SSLNegotiation = "postgres"
	cc.RequireAuth = "scram-sha-256"
	cc.ChannelBinding = "prefer"
	cc.RuntimeParams = map[string]string{
		"application_name": "bidora", "timezone": "UTC", "search_path": "pg_catalog",
		"statement_timeout":                   strconv.FormatInt(c.OperationTimeout.Milliseconds(), 10),
		"lock_timeout":                        strconv.FormatInt(c.LockTimeout.Milliseconds(), 10),
		"idle_in_transaction_session_timeout": strconv.FormatInt(c.OperationTimeout.Milliseconds(), 10),
	}
	p.MinConns, p.MinIdleConns = 0, 0
	p.MaxConns = c.MaxConnections
	p.MaxConnLifetime = c.MaxLifetime
	p.MaxConnLifetimeJitter = 0
	p.MaxConnIdleTime = c.MaxIdleTime
	p.HealthCheckPeriod = c.HealthCheckPeriod
	p.PingTimeout = c.OperationTimeout
	return p, nil
}
