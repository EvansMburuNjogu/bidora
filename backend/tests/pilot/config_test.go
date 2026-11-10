package pilot_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"maps"
	"math/big"
	"testing"
	"time"

	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/pilot"
	"github.com/google/uuid"
)

func deploymentFixture(t *testing.T) map[string]string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	dummy, err := hasher.Hash([]byte("synthetic dummy password"))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"/ca": certificate, "BIDORA_PROFILE": "external-tender-pilot", "BIDORA_ENV": "test", "BIDORA_HTTP_ADDR": "127.0.0.1:8080", "BIDORA_ALLOWED_ORIGINS": "https://app.example.test", "BIDORA_TRUSTED_PROXY_CIDRS": "127.0.0.0/8", "BIDORA_ATTEMPT_HMAC_SECRET_B64": base64.RawStdEncoding.EncodeToString(make([]byte, 32)), "BIDORA_DUMMY_PASSWORD_PHC": dummy, "BIDORA_REDIS_ADDRESS": "localhost:6379", "BIDORA_REDIS_USERNAME": "runtime", "BIDORA_REDIS_PASSWORD": "redis-password", "BIDORA_REDIS_CA_FILE": "/ca", "BIDORA_REDIS_TLS_SERVER_NAME": "localhost", "BIDORA_REDIS_PREFIX": "bidora:pilot", "BIDORA_GOOGLE_ENABLED": "true", "BIDORA_GOOGLE_CA_FILE": "/ca", "BIDORA_GOOGLE_CLIENT_ID": "synthetic-client.apps.googleusercontent.com", "BIDORA_GOOGLE_CLIENT_SECRET": "synthetic-google-secret", "BIDORA_GOOGLE_CALLBACK_URL": "https://api.example.test/v1/auth/google/callback", "BIDORA_GOOGLE_SUCCESS_REDIRECT": "https://app.example.test/auth/complete", "BIDORA_GOOGLE_FAILURE_REDIRECT": "https://app.example.test/login", "BIDORA_GOOGLE_TRANSACTION_TTL_SECONDS": "300", "BIDORA_GOOGLE_LINK_CALLBACK_URL": "https://api.example.test/v1/auth/google/link/callback", "BIDORA_GOOGLE_LINK_SUCCESS_REDIRECT": "https://app.example.test/settings/security/linked", "BIDORA_GOOGLE_LINK_FAILURE_REDIRECT": "https://app.example.test/settings/security", "BIDORA_CONTROL_HOST": "localhost", "BIDORA_CONTROL_PORT": "5432", "BIDORA_CONTROL_DATABASE": "bidora_control", "BIDORA_CONTROL_USER": "control_runtime", "BIDORA_CONTROL_PASSWORD": "control-password", "BIDORA_CONTROL_CA_FILE": "/ca", "BIDORA_TENANT_COUNT": "1", "BIDORA_TENANT_0_ID": uuid.NewString(), "BIDORA_TENANT_0_CLUSTER_ID": uuid.NewString(), "BIDORA_TENANT_0_ASSIGNMENT_GENERATION": "4", "BIDORA_TENANT_0_DATABASE_NAME": "tenant_one", "BIDORA_TENANT_0_SECRET_REF": "secret/tenant-one", "BIDORA_TENANT_0_ENDPOINT_REF": "cluster/primary", "BIDORA_TENANT_0_REGION": "test", "BIDORA_TENANT_0_POSTGRES_HOST": "localhost", "BIDORA_TENANT_0_POSTGRES_PORT": "5432", "BIDORA_TENANT_0_POSTGRES_DATABASE": "tenant_one", "BIDORA_TENANT_0_POSTGRES_USER": "tenant_runtime", "BIDORA_TENANT_0_POSTGRES_PASSWORD": "tenant-password", "BIDORA_TENANT_0_POSTGRES_CA_FILE": "/ca"}
	values["BIDORA_EMAIL_LOGIN_HMAC_SECRET_B64"] = base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	values["BIDORA_MINIO_ENDPOINT"] = "minio.example.test:9000"
	values["BIDORA_MINIO_BUCKET"] = "bidora-private-test"
	values["BIDORA_MINIO_ACCESS_KEY"] = "synthetic-minio-access"
	values["BIDORA_MINIO_SECRET_KEY"] = "synthetic-minio-secret"
	values["BIDORA_MINIO_CA_FILE"] = "/ca"
	values["BIDORA_MINIO_TLS_SERVER_NAME"] = "minio.example.test"
	values["BIDORA_MINIO_TIMEOUT_SECONDS"] = "10"
	values["BIDORA_OPENAI_API_KEY"] = "synthetic-openai-key"
	values["BIDORA_OPENAI_MODEL"] = "synthetic-model"
	values["BIDORA_OPENAI_CA_FILE"] = "/ca"
	values["BIDORA_OPENAI_TIMEOUT_SECONDS"] = "30"
	values["BIDORA_NATS_URL"] = "tls://nats.example.test:4222"
	values["BIDORA_NATS_CREDENTIALS_FILE"] = "/nats.creds"
	values["/nats.creds"] = "synthetic-nats-credentials"
	values["BIDORA_NATS_CA_FILE"] = "/ca"
	values["BIDORA_NATS_TLS_SERVER_NAME"] = "nats.example.test"
	values["BIDORA_NATS_CONNECT_TIMEOUT_SECONDS"] = "5"
	values["BIDORA_CLAMAV_SOCKET"] = "/tmp/bidora-synthetic-clamav.sock"
	values["BIDORA_CLAMAV_TIMEOUT_SECONDS"] = "10"
	values["BIDORA_CLAMAV_CHUNK_BYTES"] = "65536"
	values["BIDORA_MFA_KEY_VERSION"] = "1"
	values["BIDORA_MFA_KEY_VERSIONS"] = "1"
	values["BIDORA_MFA_KEY_1_FILE"] = "/mfa-key-1"
	mfaKey := make([]byte, 32)
	for i := range mfaKey {
		mfaKey[i] = byte(i + 1)
	}
	values["/mfa-key-1"] = base64.RawStdEncoding.EncodeToString(mfaKey)
	return values
}
func loadFixture(values map[string]string) (pilot.DeploymentConfig, error) {
	return pilot.LoadDeploymentConfig(func(key string) string { return values[key] }, func(path string) ([]byte, error) { return []byte(values[path]), nil })
}
func TestLoadDeploymentConfigRequiresExplicitIsolatedTargets(t *testing.T) {
	values := deploymentFixture(t)
	config, err := loadFixture(values)
	if err != nil {
		t.Fatal(err)
	}
	if config.Environment != "test" || config.Address != "127.0.0.1:8080" || len(config.Tenants) != 1 || config.Runtime.Google == nil || config.Runtime.Google.ClientID != "" || config.Runtime.Google.ClientSecret != "" || len(config.Runtime.SMTPVaultKey) != 0 || config.Tenants[0].Postgres.Database != "tenant_one" || config.Tenants[0].Assignment.Generation != 4 {
		t.Fatal("pilot deployment config drifted")
	}
	if config.Control.Database == config.Tenants[0].Postgres.Database {
		t.Fatal("control and tenant target were not isolated")
	}
	values["BIDORA_TENANT_0_POSTGRES_DATABASE"] = "bidora_control"
	values["BIDORA_TENANT_0_DATABASE_NAME"] = "bidora_control"
	if config, err := loadFixture(values); err == nil || config.Environment != "" {
		t.Fatal("shared control/tenant database accepted")
	}
}
func TestLoadDeploymentConfigAllowsExplicitlyDisabledGoogle(t *testing.T) {
	values := deploymentFixture(t)
	values["BIDORA_GOOGLE_ENABLED"] = "false"
	for _, key := range []string{"BIDORA_GOOGLE_CLIENT_ID", "BIDORA_GOOGLE_CLIENT_SECRET", "BIDORA_GOOGLE_CALLBACK_URL", "BIDORA_GOOGLE_SUCCESS_REDIRECT", "BIDORA_GOOGLE_FAILURE_REDIRECT", "BIDORA_GOOGLE_TRANSACTION_TTL_SECONDS", "BIDORA_GOOGLE_LINK_CALLBACK_URL", "BIDORA_GOOGLE_LINK_SUCCESS_REDIRECT", "BIDORA_GOOGLE_LINK_FAILURE_REDIRECT", "BIDORA_GOOGLE_CA_FILE"} {
		delete(values, key)
	}
	config, err := loadFixture(values)
	if err != nil {
		t.Fatal(err)
	}
	if config.Runtime.Google != nil || len(config.Runtime.SMTPVaultKey) != 0 {
		t.Fatal("explicit Google disablement changed SMTP vault state")
	}
}
func TestLoadDeploymentConfigLoadsDatabaseSMTPVaultKey(t *testing.T) {
	values := deploymentFixture(t)
	values["BIDORA_SMTP_VAULT_KEY_FILE"] = "/smtp-vault-key"
	values["BIDORA_SMTP_VAULT_KEY_VERSION"] = "1"
	values["/smtp-vault-key"] = base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	config, err := loadFixture(values)
	if err != nil || len(config.Runtime.SMTPVaultKey) != 32 || config.Runtime.SMTPVaultKeyVersion != 1 {
		t.Fatal("database SMTP vault key was not loaded", err)
	}
}

func TestLoadDeploymentConfigAllowsUnconfiguredSMTP(t *testing.T) {
	values := deploymentFixture(t)
	delete(values, "BIDORA_EMAIL_LOGIN_HMAC_SECRET_B64")
	config, err := loadFixture(values)
	if err != nil || len(config.Runtime.SMTPVaultKey) != 0 {
		t.Fatal("unconfigured SMTP was not accepted", err)
	}
}

func TestLoadDeploymentConfigFailsClosedWithoutSecurityInputs(t *testing.T) {
	base := deploymentFixture(t)
	for _, key := range []string{"BIDORA_PROFILE", "BIDORA_ENV", "BIDORA_HTTP_ADDR", "BIDORA_ALLOWED_ORIGINS", "BIDORA_TRUSTED_PROXY_CIDRS", "BIDORA_ATTEMPT_HMAC_SECRET_B64", "BIDORA_DUMMY_PASSWORD_PHC", "BIDORA_REDIS_PASSWORD", "BIDORA_GOOGLE_ENABLED", "BIDORA_GOOGLE_CALLBACK_URL", "BIDORA_GOOGLE_SUCCESS_REDIRECT", "BIDORA_GOOGLE_FAILURE_REDIRECT", "BIDORA_GOOGLE_TRANSACTION_TTL_SECONDS", "BIDORA_GOOGLE_LINK_CALLBACK_URL", "BIDORA_GOOGLE_LINK_SUCCESS_REDIRECT", "BIDORA_GOOGLE_LINK_FAILURE_REDIRECT", "BIDORA_GOOGLE_CA_FILE", "BIDORA_CONTROL_PASSWORD", "BIDORA_TENANT_0_SECRET_REF", "BIDORA_TENANT_0_POSTGRES_PASSWORD"} {
		t.Run(key, func(t *testing.T) {
			values := maps.Clone(base)
			delete(values, key)
			config, err := loadFixture(values)
			if err == nil || config.Environment != "" {
				t.Fatal("missing security input accepted", key)
			}
		})
	}
}
