package pilot

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bidora/backend/internal/eventing"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/opportunity/extractopenai"
	"bidora/backend/internal/opportunity/scanclamav"
	"bidora/backend/internal/opportunity/storageminio"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	"github.com/google/uuid"
)

const maxTenantPools = 100

type DeploymentConfig struct {
	Environment, Address string
	Runtime              Config
	Control              postgres.Config
	Tenants              []TenantDatabaseConfig
}
type TenantDatabaseConfig struct {
	Assignment tenancyapp.Assignment
	Postgres   postgres.Config
}

func (DeploymentConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED pilot deployment config]")
}

type getenvFunc func(string) string
type readFileFunc func(string) ([]byte, error)

func LoadDeploymentConfig(getenv getenvFunc, readFile readFileFunc) (DeploymentConfig, error) {
	if getenv == nil || readFile == nil {
		return DeploymentConfig{}, errors.New("pilot_config_invalid")
	}
	if getenv("BIDORA_PROFILE") != "external-tender-pilot" {
		return DeploymentConfig{}, errors.New("pilot_profile_invalid")
	}
	environment := getenv("BIDORA_ENV")
	switch environment {
	case "local", "test", "staging", "production":
	default:
		return DeploymentConfig{}, errors.New("pilot_environment_invalid")
	}
	address, err := parseListenAddress(getenv("BIDORA_HTTP_ADDR"))
	if err != nil {
		return DeploymentConfig{}, err
	}
	origins, err := parseList(getenv("BIDORA_ALLOWED_ORIGINS"), false)
	if err != nil {
		return DeploymentConfig{}, err
	}
	if _, err = browser.NewProtection(origins); err != nil {
		return DeploymentConfig{}, errors.New("pilot_origin_config_invalid")
	}
	proxies, err := parseProxies(getenv("BIDORA_TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return DeploymentConfig{}, err
	}
	if _, err = browser.NewClientIPResolver(proxies); err != nil {
		return DeploymentConfig{}, errors.New("pilot_proxy_config_invalid")
	}
	attemptSecret, err := base64.RawStdEncoding.DecodeString(getenv("BIDORA_ATTEMPT_HMAC_SECRET_B64"))
	if err != nil || len(attemptSecret) != 32 {
		return DeploymentConfig{}, errors.New("pilot_attempt_secret_invalid")
	}
	redisConfig, err := loadRedis(getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	redisValidator, err := attemptredis.NewClient(redisConfig)
	if err != nil {
		return DeploymentConfig{}, errors.New("pilot_redis_config_invalid")
	}
	if _, err = attemptredis.NewSecureController(redisValidator, getenv("BIDORA_REDIS_PREFIX")); err != nil {
		_ = redisValidator.Close()
		return DeploymentConfig{}, errors.New("pilot_redis_config_invalid")
	}
	_ = redisValidator.Close()
	var google *GoogleConfig
	googleEnabled, err := strconv.ParseBool(getenv("BIDORA_GOOGLE_ENABLED"))
	if err != nil {
		return DeploymentConfig{}, errors.New("pilot_google_config_invalid")
	}
	if googleEnabled {
		google, err = loadGoogle(getenv, readFile)
		if err != nil {
			return DeploymentConfig{}, err
		}
	}
	pspOptions, err := LoadPesaPalDeploymentOptions(getenv)
	if err != nil {
		return DeploymentConfig{}, err
	}
	smtpVaultKey, smtpVaultVersion, err := loadSMTPVaultKey(getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	notificationKey, notificationKeyVersion, err := loadNotificationFingerprintKey(getenv, readFile)
	if err != nil || len(notificationKey) != 0 && len(smtpVaultKey) == 0 {
		return DeploymentConfig{}, errors.New("pilot_notification_config_invalid")
	}
	var emailLoginSecret []byte
	if len(smtpVaultKey) != 0 {
		emailLoginSecret, err = base64.RawStdEncoding.DecodeString(getenv("BIDORA_EMAIL_LOGIN_HMAC_SECRET_B64"))
		if err != nil || len(emailLoginSecret) != 32 {
			return DeploymentConfig{}, errors.New("pilot_email_login_secret_invalid")
		}
	}
	minioConfig, err := loadMinIO(getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	// OpenAI credentials, model, TLS roots and execution limits are managed settings.
	var openAIConfig *extractopenai.Config
	natsConfig, err := loadNATS(getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	clamAVConfig, err := loadClamAV(getenv)
	if err != nil {
		return DeploymentConfig{}, err
	}
	mfaKeys, err := loadMFAKeyRing(getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	control, err := loadPostgres("BIDORA_CONTROL_", getenv, readFile)
	if err != nil {
		return DeploymentConfig{}, err
	}
	hasher, err := password.New(password.PilotPolicy())
	if err != nil || hasher.ValidatePHC(getenv("BIDORA_DUMMY_PASSWORD_PHC")) != nil {
		return DeploymentConfig{}, errors.New("pilot_dummy_hash_invalid")
	}
	count, err := strconv.Atoi(getenv("BIDORA_TENANT_COUNT"))
	if err != nil || count < 1 || count > maxTenantPools {
		return DeploymentConfig{}, errors.New("pilot_tenant_count_invalid")
	}
	tenants := make([]TenantDatabaseConfig, 0, count)
	seenTenant := map[tenancyapp.ID]struct{}{}
	seenAssignment := map[tenancyapp.Assignment]struct{}{}
	seenDatabase := map[string]struct{}{databaseTarget(control): {}}
	for index := 0; index < count; index++ {
		prefix := "BIDORA_TENANT_" + strconv.Itoa(index) + "_"
		assignment, err := loadAssignment(prefix, getenv)
		if err != nil {
			return DeploymentConfig{}, err
		}
		if _, exists := seenTenant[assignment.TenantID]; exists {
			return DeploymentConfig{}, errors.New("pilot_tenant_config_invalid")
		}
		if _, exists := seenAssignment[assignment]; exists {
			return DeploymentConfig{}, errors.New("pilot_tenant_config_invalid")
		}
		database, err := loadPostgres(prefix+"POSTGRES_", getenv, readFile)
		if err != nil {
			return DeploymentConfig{}, err
		}
		if database.Database != assignment.DatabaseName {
			return DeploymentConfig{}, errors.New("pilot_tenant_config_invalid")
		}
		target := databaseTarget(database)
		if _, exists := seenDatabase[target]; exists {
			return DeploymentConfig{}, errors.New("pilot_tenant_config_invalid")
		}
		seenDatabase[target] = struct{}{}
		seenTenant[assignment.TenantID] = struct{}{}
		seenAssignment[assignment] = struct{}{}
		tenants = append(tenants, TenantDatabaseConfig{Assignment: assignment, Postgres: database})
	}
	return DeploymentConfig{Environment: environment, Address: address, Runtime: Config{PesaPal: pspOptions, PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: getenv("BIDORA_DUMMY_PASSWORD_PHC"), AttemptHMACSecret: attemptSecret, EmailLoginHMACSecret: emailLoginSecret, Redis: redisConfig, RedisPrefix: getenv("BIDORA_REDIS_PREFIX"), AllowedOrigins: origins, TrustedProxies: proxies, Google: google, SMTPVaultKey: smtpVaultKey, SMTPVaultKeyVersion: smtpVaultVersion, NotificationFingerprintKey: notificationKey, NotificationFingerprintKeyVersion: notificationKeyVersion, MinIO: minioConfig, OpenAI: openAIConfig, NATS: natsConfig, DocumentMeasurerPath: getenv("BIDORA_DOCUMENT_MEASURER_PATH"), PDFToTextPath: getenv("BIDORA_PDFTOTEXT_PATH"), ClamAV: clamAVConfig, MFAKeyRing: mfaKeys}, Control: control, Tenants: tenants}, nil
}
func loadMFAKeyRing(getenv getenvFunc, readFile readFileFunc) (*mfa.KeyRing, error) {
	current, e := strconv.Atoi(getenv("BIDORA_MFA_KEY_VERSION"))
	if e != nil || current < 1 {
		return nil, errors.New("pilot_mfa_key_invalid")
	}
	versions, e := parseList(getenv("BIDORA_MFA_KEY_VERSIONS"), false)
	if e != nil {
		return nil, errors.New("pilot_mfa_key_invalid")
	}
	keys := map[int][32]byte{}
	for _, text := range versions {
		v, e := strconv.Atoi(text)
		if e != nil || v < 1 {
			return nil, errors.New("pilot_mfa_key_invalid")
		}
		raw, e := readFile(getenv("BIDORA_MFA_KEY_" + strconv.Itoa(v) + "_FILE"))
		if e != nil {
			return nil, errors.New("pilot_mfa_key_invalid")
		}
		decoded, e := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if e != nil {
			decoded, e = base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		}
		if e != nil || len(decoded) != 32 {
			return nil, errors.New("pilot_mfa_key_invalid")
		}
		var key [32]byte
		copy(key[:], decoded)
		for i := range decoded {
			decoded[i] = 0
		}
		keys[v] = key
	}
	ring, e := mfa.NewKeyRing(current, keys)
	if e != nil {
		return nil, errors.New("pilot_mfa_key_invalid")
	}
	return ring, nil
}

func loadNATS(getenv getenvFunc, readFile readFileFunc) (*eventing.NATSConfig, error) {
	roots, err := loadRoots(getenv("BIDORA_NATS_CA_FILE"), readFile)
	if err != nil {
		return nil, errors.New("pilot_nats_config_invalid")
	}
	seconds, err := strconv.ParseInt(getenv("BIDORA_NATS_CONNECT_TIMEOUT_SECONDS"), 10, 64)
	if err != nil || seconds < 1 || seconds > 30 {
		return nil, errors.New("pilot_nats_config_invalid")
	}
	config := &eventing.NATSConfig{URL: getenv("BIDORA_NATS_URL"), CredentialsFile: getenv("BIDORA_NATS_CREDENTIALS_FILE"), ServerName: getenv("BIDORA_NATS_TLS_SERVER_NAME"), RootCAs: roots, ConnectTimeout: time.Duration(seconds) * time.Second}
	return config, nil
}
func loadClamAV(getenv getenvFunc) (*scanclamav.Config, error) {
	seconds, err := strconv.ParseInt(getenv("BIDORA_CLAMAV_TIMEOUT_SECONDS"), 10, 64)
	if err != nil || seconds < 1 || seconds > 120 {
		return nil, errors.New("pilot_clamav_config_invalid")
	}
	chunk, err := strconv.Atoi(getenv("BIDORA_CLAMAV_CHUNK_BYTES"))
	if err != nil {
		return nil, errors.New("pilot_clamav_config_invalid")
	}
	config := &scanclamav.Config{SocketPath: getenv("BIDORA_CLAMAV_SOCKET"), Timeout: time.Duration(seconds) * time.Second, ChunkBytes: chunk}
	if _, err = scanclamav.New(*config); err != nil {
		return nil, errors.New("pilot_clamav_config_invalid")
	}
	return config, nil
}
func loadMinIO(getenv getenvFunc, readFile readFileFunc) (*storageminio.Config, error) {
	roots, err := loadRoots(getenv("BIDORA_MINIO_CA_FILE"), readFile)
	if err != nil {
		return nil, errors.New("pilot_minio_config_invalid")
	}
	timeoutSeconds, err := strconv.ParseInt(getenv("BIDORA_MINIO_TIMEOUT_SECONDS"), 10, 64)
	if err != nil || timeoutSeconds < 1 || timeoutSeconds > 30 {
		return nil, errors.New("pilot_minio_config_invalid")
	}
	config := &storageminio.Config{Endpoint: getenv("BIDORA_MINIO_ENDPOINT"), Bucket: getenv("BIDORA_MINIO_BUCKET"), AccessKey: getenv("BIDORA_MINIO_ACCESS_KEY"), SecretKey: getenv("BIDORA_MINIO_SECRET_KEY"), RootCAs: roots, ServerName: getenv("BIDORA_MINIO_TLS_SERVER_NAME"), Timeout: time.Duration(timeoutSeconds) * time.Second}
	if _, err = storageminio.New(*config); err != nil {
		return nil, errors.New("pilot_minio_config_invalid")
	}
	return config, nil
}

// Notification fingerprints use independent, explicitly supplied key material;
// they never reuse the SMTP encryption key or expose recipient/content values.
func loadNotificationFingerprintKey(getenv getenvFunc, readFile readFileFunc) ([]byte, int32, error) {
	path, versionText := getenv("BIDORA_NOTIFICATION_FINGERPRINT_KEY_FILE"), getenv("BIDORA_NOTIFICATION_FINGERPRINT_KEY_VERSION")
	if path == "" && versionText == "" {
		return nil, 0, nil
	}
	if path == "" {
		return nil, 0, errors.New("pilot_notification_config_invalid")
	}
	raw, err := readFile(path)
	if err != nil {
		return nil, 0, errors.New("pilot_notification_config_invalid")
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	}
	version, versionErr := strconv.ParseInt(versionText, 10, 32)
	if err != nil || len(key) != 32 || versionErr != nil || version < 1 {
		return nil, 0, errors.New("pilot_notification_config_invalid")
	}
	return key, int32(version), nil
}

func loadSMTPVaultKey(getenv getenvFunc, readFile readFileFunc) ([]byte, int32, error) {
	path := getenv("BIDORA_SMTP_VAULT_KEY_FILE")
	if path == "" {
		return nil, 0, nil
	}
	raw, err := readFile(path)
	if err != nil {
		return nil, 0, errors.New("pilot_smtp_vault_config_invalid")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	}
	version, versionErr := strconv.ParseInt(getenv("BIDORA_SMTP_VAULT_KEY_VERSION"), 10, 32)
	if err != nil || len(decoded) != 32 || versionErr != nil || version < 1 {
		return nil, 0, errors.New("pilot_smtp_vault_config_invalid")
	}
	return decoded, int32(version), nil
}
func LoadGoogleDeploymentConfig(getenv func(string) string, readFile func(string) ([]byte, error)) (*GoogleConfig, error) {
	if getenv == nil || readFile == nil {
		return nil, errors.New("pilot_google_config_invalid")
	}
	return loadGoogle(getenv, readFile)
}

func loadGoogle(getenv getenvFunc, readFile readFileFunc) (*GoogleConfig, error) {
	roots, err := loadRoots(getenv("BIDORA_GOOGLE_CA_FILE"), readFile)
	if err != nil {
		return nil, errors.New("pilot_google_config_invalid")
	}
	ttlSeconds, err := strconv.ParseInt(getenv("BIDORA_GOOGLE_TRANSACTION_TTL_SECONDS"), 10, 64)
	if err != nil || ttlSeconds < 60 || ttlSeconds > 900 {
		return nil, errors.New("pilot_google_config_invalid")
	}
	config := &GoogleConfig{CallbackURL: getenv("BIDORA_GOOGLE_CALLBACK_URL"), SuccessRedirect: getenv("BIDORA_GOOGLE_SUCCESS_REDIRECT"), FailureRedirect: getenv("BIDORA_GOOGLE_FAILURE_REDIRECT"), LinkCallbackURL: getenv("BIDORA_GOOGLE_LINK_CALLBACK_URL"), LinkSuccessRedirect: getenv("BIDORA_GOOGLE_LINK_SUCCESS_REDIRECT"), LinkFailureRedirect: getenv("BIDORA_GOOGLE_LINK_FAILURE_REDIRECT"), TransactionTTL: time.Duration(ttlSeconds) * time.Second, Transport: googleoidc.TransportConfig{RootCAs: roots, DialTimeout: 3 * time.Second, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 5 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 8, MaxIdleConnsPerHost: 4, MaxConnsPerHost: 8}}
	transport, err := googleoidc.NewTransport(config.Transport)
	if err != nil {
		return nil, errors.New("pilot_google_config_invalid")
	}
	defer transport.CloseIdleConnections()
	// Client credentials and enable/signup policy are now database-managed.
	// Deployment owns only fixed callbacks, trust roots and transport bounds.
	transaction, err := googleoidc.GenerateTransactionWithRandom(strings.NewReader(strings.Repeat("g", googleoidc.TransactionSecretBytes*4)))
	if err != nil {
		return nil, errors.New("pilot_google_config_invalid")
	}
	if _, err = googleoidc.BuildAuthorizationRequest(googleoidc.AuthorizationConfig{ClientID: "1-managed.apps.googleusercontent.com", RedirectURI: config.LinkCallbackURL}, transaction); err != nil {
		return nil, errors.New("pilot_google_config_invalid")
	}
	if _, err = googleoidc.BuildAuthorizationRequest(googleoidc.AuthorizationConfig{ClientID: "1-managed.apps.googleusercontent.com", RedirectURI: config.CallbackURL}, transaction); err != nil || !fixedHTTPSRedirect(config.SuccessRedirect) || !fixedHTTPSRedirect(config.FailureRedirect) || config.SuccessRedirect == config.FailureRedirect || !fixedHTTPSRedirect(config.LinkSuccessRedirect) || !fixedHTTPSRedirect(config.LinkFailureRedirect) || config.LinkSuccessRedirect == config.LinkFailureRedirect {
		return nil, errors.New("pilot_google_config_invalid")
	}
	return config, nil
}
func fixedHTTPSRedirect(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.String() == value
}

func loadPostgres(prefix string, getenv getenvFunc, readFile readFileFunc) (postgres.Config, error) {
	port, err := parsePort(getenv(prefix + "PORT"))
	if err != nil {
		return postgres.Config{}, errors.New("pilot_postgres_config_invalid")
	}
	roots, err := loadRoots(getenv(prefix+"CA_FILE"), readFile)
	if err != nil {
		return postgres.Config{}, errors.New("pilot_postgres_config_invalid")
	}
	config := postgres.Config{Host: getenv(prefix + "HOST"), Port: port, Database: getenv(prefix + "DATABASE"), User: getenv(prefix + "USER"), Password: getenv(prefix + "PASSWORD"), RootCAs: roots, MaxConnections: 8, ConnectTimeout: 5 * time.Second, OperationTimeout: 5 * time.Second, LockTimeout: time.Second, MaxLifetime: time.Hour, MaxIdleTime: 5 * time.Minute, HealthCheckPeriod: 30 * time.Second}
	if config.Validate() != nil {
		return postgres.Config{}, errors.New("pilot_postgres_config_invalid")
	}
	return config, nil
}
func loadRedis(getenv getenvFunc, readFile readFileFunc) (attemptredis.Config, error) {
	roots, err := loadRoots(getenv("BIDORA_REDIS_CA_FILE"), readFile)
	if err != nil {
		return attemptredis.Config{}, errors.New("pilot_redis_config_invalid")
	}
	serverName := getenv("BIDORA_REDIS_TLS_SERVER_NAME")
	username := getenv("BIDORA_REDIS_USERNAME")
	if serverName == "" || strings.TrimSpace(serverName) != serverName || username == "" || len(username) > 128 || strings.TrimSpace(username) != username {
		return attemptredis.Config{}, errors.New("pilot_redis_config_invalid")
	}
	config := attemptredis.Config{Address: getenv("BIDORA_REDIS_ADDRESS"), Username: username, Password: getenv("BIDORA_REDIS_PASSWORD"), TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName, RootCAs: roots}, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, PoolSize: 16}
	return config, nil
}
func loadAssignment(prefix string, getenv getenvFunc) (tenancyapp.Assignment, error) {
	tenant, err := uuid.Parse(getenv(prefix + "ID"))
	if err != nil {
		return tenancyapp.Assignment{}, errors.New("pilot_tenant_config_invalid")
	}
	cluster, err := uuid.Parse(getenv(prefix + "CLUSTER_ID"))
	if err != nil {
		return tenancyapp.Assignment{}, errors.New("pilot_tenant_config_invalid")
	}
	generation, err := strconv.ParseInt(getenv(prefix+"ASSIGNMENT_GENERATION"), 10, 64)
	if err != nil {
		return tenancyapp.Assignment{}, errors.New("pilot_tenant_config_invalid")
	}
	assignment := tenancyapp.Assignment{TenantID: tenancyapp.ID(tenant), ClusterID: tenancyapp.ID(cluster), Generation: generation, DatabaseName: getenv(prefix + "DATABASE_NAME"), SecretRef: getenv(prefix + "SECRET_REF"), EndpointRef: getenv(prefix + "ENDPOINT_REF"), Region: getenv(prefix + "REGION")}
	if !assignment.Valid() {
		return tenancyapp.Assignment{}, errors.New("pilot_tenant_config_invalid")
	}
	return assignment, nil
}
func loadRoots(path string, readFile readFileFunc) (*x509.CertPool, error) {
	if path == "" || strings.TrimSpace(path) != path {
		return nil, errors.New("certificate_invalid")
	}
	pem, err := readFile(path)
	if err != nil || len(pem) == 0 || len(pem) > 1024*1024 {
		return nil, errors.New("certificate_invalid")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("certificate_invalid")
	}
	return roots, nil
}
func parseListenAddress(value string) (string, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host == "" || port == "" {
		return "", errors.New("pilot_http_address_invalid")
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" || ip.IsMulticast() {
		return "", errors.New("pilot_http_address_invalid")
	}
	if _, err = parsePort(port); err != nil {
		return "", errors.New("pilot_http_address_invalid")
	}
	return value, nil
}
func parsePort(value string) (uint16, error) {
	number, err := strconv.ParseUint(value, 10, 16)
	if err != nil || number == 0 {
		return 0, errors.New("port_invalid")
	}
	return uint16(number), nil
}
func parseList(value string, allowNone bool) ([]string, error) {
	if allowNone && value == "none" {
		return []string{}, nil
	}
	if value == "" {
		return nil, errors.New("list_invalid")
	}
	parts := strings.Split(value, ",")
	seen := map[string]struct{}{}
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return nil, errors.New("list_invalid")
		}
		if _, exists := seen[part]; exists {
			return nil, errors.New("list_invalid")
		}
		seen[part] = struct{}{}
	}
	return parts, nil
}
func databaseTarget(config postgres.Config) string {
	return config.Host + ":" + strconv.Itoa(int(config.Port)) + "/" + config.Database
}
func parseProxies(value string) ([]netip.Prefix, error) {
	parts, err := parseList(value, false)
	if err != nil {
		return nil, errors.New("pilot_proxy_config_invalid")
	}
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(part)
		if err != nil || prefix != prefix.Masked() {
			return nil, errors.New("pilot_proxy_config_invalid")
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
