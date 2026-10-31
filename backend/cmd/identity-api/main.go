package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"log"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"bidora/backend/internal/health"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	staging := os.Getenv("BIDORA_ENV") == "staging"
	if environment := os.Getenv("BIDORA_ENV"); environment != "" && environment != "local" && !staging {
		return errors.New("identity_environment_invalid")
	}
	address := os.Getenv("BIDORA_LOCAL_HTTP_ADDR")
	if staging {
		address = os.Getenv("BIDORA_HTTP_ADDR")
	}
	if address == "" {
		address = "127.0.0.1:8080"
	}
	bind, err := netip.ParseAddrPort(address)
	if err != nil || (!staging && !bind.Addr().IsLoopback()) || bind.Port() == 0 {
		return errors.New("local_http_address_invalid")
	}
	pgRoots, err := roots(os.Getenv("BIDORA_TEST_POSTGRES_CA_FILE"))
	if err != nil {
		return errors.New("postgres_roots_invalid")
	}
	pg, err := pgConfig(os.Getenv("BIDORA_TEST_CONTROL_RUNTIME_DSN"), pgRoots)
	if err != nil {
		return err
	}
	pool, err := postgres.New(pg)
	if err != nil {
		return err
	}
	defer pool.Close(context.Background())
	redisRoots, err := roots(os.Getenv("BIDORA_TEST_REDIS_CA_FILE"))
	if err != nil {
		return errors.New("redis_roots_invalid")
	}
	attemptKey, err := secret("BIDORA_LOCAL_ATTEMPT_SECRET_B64")
	if err != nil {
		return err
	}
	emailKey, err := secret("BIDORA_LOCAL_EMAIL_LOGIN_HMAC_SECRET_B64")
	if err != nil {
		return err
	}
	mfaKey, err := secret("BIDORA_LOCAL_MFA_KEY_B64")
	if err != nil {
		return err
	}
	vaultKey, err := secret("BIDORA_LOCAL_SMTP_VAULT_KEY_B64")
	if err != nil {
		return err
	}
	version64, err := strconv.ParseInt(os.Getenv("BIDORA_LOCAL_SMTP_VAULT_KEY_VERSION"), 10, 32)
	if err != nil || version64 < 1 {
		return errors.New("vault_version_invalid")
	}
	var key [32]byte
	copy(key[:], mfaKey)
	ring, err := mfa.NewKeyRing(1, map[int][32]byte{1: key})
	if err != nil {
		return err
	}
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		return err
	}
	dummy, err := hasher.Hash([]byte("local synthetic dummy password"))
	if err != nil {
		return err
	}
	var notificationKey []byte
	var notificationVersion int64
	if os.Getenv("BIDORA_LOCAL_NOTIFICATION_FINGERPRINT_KEY_B64") != "" || os.Getenv("BIDORA_LOCAL_NOTIFICATION_FINGERPRINT_KEY_VERSION") != "" {
		notificationKey, err = secret("BIDORA_LOCAL_NOTIFICATION_FINGERPRINT_KEY_B64")
		if err != nil {
			return errors.New("notification_fingerprint_config_invalid")
		}
		notificationVersion, err = strconv.ParseInt(os.Getenv("BIDORA_LOCAL_NOTIFICATION_FINGERPRINT_KEY_VERSION"), 10, 32)
		if err != nil || notificationVersion < 1 {
			return errors.New("notification_fingerprint_config_invalid")
		}
	}
	var proxies []netip.Prefix
	if raw := os.Getenv("BIDORA_LOCAL_TRUSTED_PROXY_CIDRS"); raw == "" {
		return errors.New("trusted_proxy_cidrs_required")
	} else {
		for _, entry := range strings.Split(raw, ",") {
			prefix, parseErr := netip.ParsePrefix(strings.TrimSpace(entry))
			if parseErr != nil || prefix != prefix.Masked() {
				return errors.New("trusted_proxy_cidrs_invalid")
			}
			proxies = append(proxies, prefix)
		}
	}
	pspOptions, err := pilot.LoadPesaPalDeploymentOptions(os.Getenv)
	if err != nil {
		log.Fatal("PesaPal deployment configuration is invalid")
	}
	origins := []string{"http://127.0.0.1:3000"}
	redisName := "localhost"
	redisPrefix := "bidora:smtp001:"
	if staging {
		origins = strings.Split(os.Getenv("BIDORA_ALLOWED_ORIGINS"), ",")
		for index, origin := range origins {
			origin = strings.TrimSpace(origin)
			u, e := url.Parse(origin)
			if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				return errors.New("sandbox_origin_invalid")
			}
			origins[index] = origin
		}
		redisName = os.Getenv("BIDORA_REDIS_TLS_SERVER_NAME")
		if redisName == "" {
			return errors.New("redis_tls_name_required")
		}
		redisPrefix = "bidora:sandbox:"
	}
	cfg := pilot.Config{PesaPal: pspOptions, PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummy, AttemptHMACSecret: attemptKey, EmailLoginHMACSecret: emailKey, Redis: attemptredis.Config{Address: os.Getenv("BIDORA_TEST_REDIS_ADDR"), Username: os.Getenv("BIDORA_TEST_REDIS_USERNAME"), Password: os.Getenv("BIDORA_TEST_REDIS_PASSWORD"), TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: redisName, RootCAs: redisRoots}, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, PoolSize: 8}, RedisPrefix: redisPrefix, AllowedOrigins: origins, TrustedProxies: proxies, MFAKeyRing: ring, SMTPVaultKey: vaultKey, SMTPVaultKeyVersion: int32(version64), NotificationFingerprintKey: notificationKey, NotificationFingerprintKeyVersion: int32(notificationVersion), DevelopmentLoopbackHTTP: !staging}
	if os.Getenv("BIDORA_GOOGLE_CALLBACK_URL") != "" {
		cfg.Google, err = pilot.LoadGoogleDeploymentConfig(os.Getenv, os.ReadFile)
		if err != nil {
			return err
		}
	}
	state := &health.State{}
	runtime, err := pilot.BuildIdentityRuntime(cfg, state, pool)
	if err != nil {
		return err
	}
	defer runtime.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	probe, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
	err = runtime.Probe(probe)
	cancelProbe()
	if err != nil {
		return err
	}
	state.Start()
	done := make(chan error, 1)
	go func() { done <- runtime.App.Listen(address) }()
	log.Print("identity and Platform API listening")
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		state.Drain()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return runtime.App.ShutdownWithContext(shutdown)
	}
}
func secret(name string) ([]byte, error) {
	text := strings.TrimSpace(os.Getenv(name))
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(text)
	}
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(text)
	}
	if err != nil || len(raw) != 32 {
		return nil, errors.New(name + " invalid")
	}
	return raw, nil
}
func roots(path string) (*x509.CertPool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(raw) {
		return nil, errors.New("roots_invalid")
	}
	return p, nil
}
func pgConfig(raw string, roots *x509.CertPool) (postgres.Config, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" || u.User == nil {
		return postgres.Config{}, errors.New("postgres_dsn_invalid")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return postgres.Config{}, errors.New("postgres_dsn_invalid")
	}
	password, ok := u.User.Password()
	if !ok {
		return postgres.Config{}, errors.New("postgres_dsn_invalid")
	}
	return postgres.Config{Host: u.Hostname(), Port: uint16(port), Database: strings.TrimPrefix(u.Path, "/"), User: u.User.Username(), Password: password, RootCAs: roots, MaxConnections: 8, ConnectTimeout: 3 * time.Second, OperationTimeout: 5 * time.Second, LockTimeout: time.Second, MaxLifetime: 30 * time.Minute, MaxIdleTime: 5 * time.Minute, HealthCheckPeriod: 30 * time.Second}, nil
}
