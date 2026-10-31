package attemptredis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Config struct {
	Address, Username, Password            string
	TLS                                    *tls.Config
	DialTimeout, ReadTimeout, WriteTimeout time.Duration
	PoolSize                               int
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED redis attempt config]") }
func NewClient(config Config) (*ClientRunner, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil || host == "" || strings.TrimSpace(host) != host || strings.ContainsAny(host, "/\\") || port == "" {
		return nil, errors.New("redis_attempt_config_invalid")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 || config.Password == "" || config.TLS == nil || config.TLS.InsecureSkipVerify || config.TLS.ServerName == "" || config.TLS.MaxVersion != 0 && config.TLS.MaxVersion < tls.VersionTLS12 || config.TLS.MinVersion != 0 && config.TLS.MinVersion < tls.VersionTLS12 || config.TLS.MaxVersion != 0 && config.TLS.MinVersion != 0 && config.TLS.MinVersion > config.TLS.MaxVersion || config.DialTimeout <= 0 || config.ReadTimeout <= 0 || config.WriteTimeout <= 0 || config.DialTimeout > 10*time.Second || config.ReadTimeout > 10*time.Second || config.WriteTimeout > 10*time.Second || config.PoolSize < 1 || config.PoolSize > 100 {
		return nil, errors.New("redis_attempt_config_invalid")
	}
	tlsConfig := config.TLS.Clone()
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	client := redis.NewClient(&redis.Options{Addr: config.Address, Username: config.Username, Password: config.Password, DB: 0, MaxRetries: -1, DialTimeout: config.DialTimeout, ReadTimeout: config.ReadTimeout, WriteTimeout: config.WriteTimeout, PoolSize: config.PoolSize, TLSConfig: tlsConfig})
	return &ClientRunner{client: client}, nil
}
func (r *ClientRunner) Ping(ctx context.Context) error {
	if ctx == nil || r == nil || r.client == nil {
		return errors.New("redis_attempt_unavailable")
	}
	if err := r.client.Ping(ctx).Err(); err != nil {
		return &availabilityError{cause: err}
	}
	return nil
}

type availabilityError struct{ cause error }

func (e *availabilityError) Error() string                 { return "redis_attempt_unavailable" }
func (e *availabilityError) Unwrap() error                 { return e.cause }
func (e *availabilityError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
func (r *ClientRunner) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	r.closeOnce.Do(func() { r.closeErr = r.client.Close() })
	return r.closeErr
}
