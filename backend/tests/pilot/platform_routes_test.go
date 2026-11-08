package pilot_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/health"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
)

// Construct the real composition with lazy, unreachable dependency clients.
// Route inventory is source/composition evidence, not authenticated DB evidence.
func TestIdentityRuntimeMountsPlatformOperations(t *testing.T) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "PG") {
			t.Setenv(key, "")
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	pool, err := postgres.New(postgres.Config{Host: "127.0.0.1", Port: 1, Database: "synthetic_control", User: "synthetic_runtime", Password: "synthetic_password", RootCAs: roots, MaxConnections: 2, ConnectTimeout: time.Second, OperationTimeout: time.Second, LockTimeout: 100 * time.Millisecond, MaxLifetime: time.Minute, MaxIdleTime: 10 * time.Second, HealthCheckPeriod: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close(context.Background())
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	dummy, err := hasher.Hash([]byte("synthetic route inventory password"))
	if err != nil {
		t.Fatal(err)
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	ring, err := mfa.NewKeyRing(1, map[int][32]byte{1: secret})
	if err != nil {
		t.Fatal(err)
	}
	cfg := pilot.Config{PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummy, AttemptHMACSecret: secret[:], EmailLoginHMACSecret: secret[:], MFAKeyRing: ring, AllowedOrigins: []string{"https://app.example.test"}, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, RedisPrefix: "bidora:route-test:", Redis: attemptredis.Config{Address: "127.0.0.1:1", Password: "synthetic_password", TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots}, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 1}}
	for _, withVault := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_delivery", true: "with_vault_and_notification_evidence"}[withVault], func(t *testing.T) {
			config := cfg
			if withVault {
				config.SMTPVaultKey = secret[:]
				config.SMTPVaultKeyVersion = 1
				config.NotificationFingerprintKey = append([]byte(nil), secret[:]...)
				config.NotificationFingerprintKey[0] ^= 1
				config.NotificationFingerprintKeyVersion = 1
			}
			runtime, err := pilot.BuildIdentityRuntime(config, &health.State{}, pool)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if withVault {
				invalid := config
				invalid.NotificationFingerprintKey = config.SMTPVaultKey
				if bad, err := pilot.BuildIdentityRuntime(invalid, &health.State{}, pool); err == nil {
					bad.Close()
					t.Fatal("notification fingerprints reused the SMTP encryption key")
				}
			}
			routes := map[string]int{}
			for _, route := range runtime.App.GetRoutes(true) {
				routes[route.Method+" "+route.Path]++
			}
			required := []string{
				"GET /v1/platform/roles", "GET /v1/platform/administrators", "POST /v1/platform/administrators/search",
				"GET /v1/platform/administrators/:user_id", "GET /v1/platform/administrators/:user_id/assignment-history",
				"POST /v1/platform/administrators/:user_id/sessions/revoke", "GET /v1/platform/administrator-changes", "POST /v1/platform/administrator-changes",
				"GET /v1/platform/administrator-changes/:change_id", "POST /v1/platform/administrator-changes/:change_id/approve", "POST /v1/platform/administrator-changes/:change_id/reject",
				"GET /v1/platform/tenants", "GET /v1/platform/tenants/:tenant_id", "GET /v1/shared/commercial/plans", "GET /v1/platform/dashboard",
				"GET /v1/platform/commercial/ai-usage", "GET /v1/platform/commercial/invoices", "GET /v1/platform/commercial/payments", "POST /v1/platform/commercial/payments/:payment_id/reconciliation",
				"GET /v1/platform/commercial/plans/management", "POST /v1/platform/commercial/plans/:plan_key/drafts", "POST /v1/platform/commercial/plans/:plan_key/publish",
				"GET /v1/platform/commercial/subscriptions", "GET /v1/platform/commercial/subscriptions/:tenant_id", "POST /v1/platform/commercial/subscriptions/:tenant_id/cancellation", "POST /v1/platform/commercial/subscriptions/:tenant_id/cancellation/withdraw",
				"GET /v1/platform/commercial/financial-adjustments", "GET /v1/platform/commercial/financial-adjustments/:change_id", "POST /v1/platform/commercial/financial-adjustments", "POST /v1/platform/commercial/financial-adjustments/:change_id/decision",
				"GET /v1/platform/commercial/payment-reversals", "GET /v1/platform/commercial/payment-reversals/:payment_id",
				"GET /v1/platform/notifications/attempts", "GET /v1/platform/audit/actions", "GET /v1/platform/audit/actions/:audit_id",
				"GET /v1/shared/platform/information", "GET /v1/platform/security/session-policy", "PUT /v1/platform/security/session-policy",
				"GET /v1/platform/information", "PUT /v1/platform/information", "GET /v1/platform/integration-settings/:setting_kind",
				"GET /v1/platform/integration-settings/:setting_kind/versions", "GET /v1/platform/integration-setting-changes/:change_id",
				"GET /v1/platform/settings/google", "PUT /v1/platform/settings/google", "GET /v1/platform/settings/google/history", "POST /v1/platform/settings/google/test",
				"GET /v1/platform/notification-templates", "GET /v1/platform/notification-templates/:template_key", "PUT /v1/platform/notification-templates/:template_key",
				"GET /v1/platform/notification-templates/:template_key/history", "POST /v1/platform/notification-templates/:template_key/preview", "POST /v1/platform/notification-templates/:template_key/test",
				"GET /v1/platform/countries", "POST /v1/platform/countries", "PUT /v1/platform/countries/:code", "GET /v1/platform/countries/:code/history",
				"GET /v1/platform/operational-policy", "PUT /v1/platform/operational-policy", "GET /v1/platform/operational-policy/history",
			}
			if withVault {
				required = append(required, "POST /v1/platform/administrator-invitations", "POST /v1/platform/administrator-invitations/:invitation_id/reissue", "GET /v1/platform/administrator-invitations", "POST /v1/platform/integration-settings/:setting_kind/versions", "POST /v1/platform/integration-settings/:setting_kind/versions/:version_id/test", "POST /v1/platform/integration-settings/:setting_kind/versions/:version_id/activation-proposals", "POST /v1/platform/integration-setting-changes/:change_id/approve", "POST /v1/platform/integration-setting-changes/:change_id/reject")
			}
			for _, route := range required {
				if routes[route] != 1 {
					t.Errorf("expected one registration for %s, got %d", route, routes[route])
				}
			}
			for _, retired := range []string{"POST /v1/auth/mfa/totp/enroll", "POST /v1/auth/mfa/totp/confirm", "POST /v1/auth/mfa/step-up", "POST /v1/auth/mfa/recovery-codes/regenerate", "POST /v1/auth/mfa/totp/replace"} {
				if routes[retired] != 0 {
					t.Errorf("retired authenticator route still mounted: %s", retired)
				}
			}
			if routes["POST /v1/platform/notifications/attempts/:attempt_id/retry"] != 0 {
				t.Error("generic notification retry must remain unmounted")
			}
			for route, count := range routes {
				if strings.HasPrefix(route, "GET /v1/platform/") || strings.HasPrefix(route, "POST /v1/platform/") || strings.HasPrefix(route, "PUT /v1/platform/") || strings.HasPrefix(route, "DELETE /v1/platform/") {
					if count != 1 {
						t.Errorf("duplicate Platform route %s registered %d times", route, count)
					}
				}
			}
		})
	}
}

func TestIdentityRuntimeProbeRejectsIncompleteComposition(t *testing.T) {
	if ((*pilot.IdentityRuntime)(nil)).Probe(context.Background()) == nil || (&pilot.IdentityRuntime{}).Probe(nil) == nil {
		t.Fatal("incomplete runtime passed readiness")
	}
}
