// Command sandbox-owner-bootstrap is an explicit deployment-operator tool, not
// an HTTP route or runtime SMTP fallback. It cannot activate managed SMTP or
// bypass email verification, authenticator enrollment, or the one-owner latch.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	identitypersistence "bidora/backend/internal/identity/persistence"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type input struct {
	Email        string `json:"email"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     uint16 `json:"smtp_port"`
	SenderName   string `json:"sender_name"`
}

func (input) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED bootstrap input]") }

type clock struct{}

func (clock) Now() time.Time { return time.Now().UTC() }

type ids struct{}

func (ids) NewID() (identityapp.ID, error) {
	id, err := uuid.NewRandom()
	return identityapp.ID(id), err
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		// Every error returned by run is a stable, content-free operator error.
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) != 2 || (args[0] != "verify-email" && args[0] != "promote" && args[0] != "seal-login-smtp") {
		return errors.New("bootstrap_arguments_invalid")
	}
	if os.Getenv("BIDORA_ENV") != "staging" {
		return errors.New("bootstrap_environment_not_sandbox")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	config, err := databaseConfig()
	if err != nil {
		return err
	}
	file, err := os.Open(args[1])
	if err != nil {
		return errors.New("bootstrap_private_input_unavailable")
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() > 4096 {
		return errors.New("bootstrap_private_input_invalid")
	}
	var in input
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		return errors.New("bootstrap_private_input_invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("bootstrap_private_input_invalid")
	}
	address, err := email.Parse(in.Email)
	if err != nil {
		return errors.New("bootstrap_email_invalid")
	}
	pool, err := postgres.New(config)
	if err != nil {
		return errors.New("bootstrap_database_unavailable")
	}
	defer pool.Close(context.Background())
	// This is only a preflight. The authoritative promotion use case rechecks and
	// locks the governance row and all identity eligibility inside its transaction.
	err = pool.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		var consumed bool
		if err := db.QueryRow(txCtx, `SELECT bootstrap_consumed FROM platform_administration.governance_state WHERE singleton=true`).Scan(&consumed); err != nil {
			return err
		}
		if consumed && args[0] != "seal-login-smtp" {
			return errors.New("consumed")
		}
		return nil
	})
	if err != nil {
		return errors.New("bootstrap_unavailable_or_consumed")
	}
	reader, err := identitypersistence.NewPasswordReader(pool)
	if err != nil {
		return errors.New("bootstrap_identity_unavailable")
	}
	account, err := reader.LookupPasswordCredential(ctx, address.Lookup())
	if err != nil || account.UserID.IsZero() || account.DisabledAt != nil {
		return errors.New("bootstrap_identity_unavailable")
	}
	if args[0] == "seal-login-smtp" {
		return sealLoginSMTP(ctx, pool, account.UserID, in)
	}
	if args[0] == "promote" {
		store, err := platformpersistence.NewStore(pool)
		if err != nil {
			return errors.New("bootstrap_owner_unavailable")
		}
		service, err := platformapp.NewBootstrapService(store)
		if err != nil {
			return errors.New("bootstrap_owner_unavailable")
		}
		_, err = service.Bootstrap(ctx, platformapp.BootstrapCommit{OwnerUserID: platformapp.ID(account.UserID), AssignmentID: platformapp.ID(uuid.New()), AuditID: platformapp.ID(uuid.New()), OutboxID: platformapp.ID(uuid.New()), CorrelationID: platformapp.ID(uuid.New()), OccurredAt: time.Now().UTC()})
		if err != nil {
			return errors.New("bootstrap_owner_not_eligible_or_unavailable")
		}
		fmt.Println("bootstrap_owner_promoted audit_and_outbox_committed=true sessions_revoked=true")
		return nil
	}
	if account.EmailVerifiedAt != nil {
		return errors.New("bootstrap_email_already_verified")
	}
	// These are explicit deployment inputs, scoped to this one message and this
	// unconsumed sandbox bootstrap. They never become an ambient server fallback.
	if in.SMTPHost != "smtp.zoho.com" || in.SMTPPort != 587 {
		return errors.New("bootstrap_smtp_endpoint_not_allowed")
	}
	sender, err := email.Parse(in.SMTPUsername)
	if err != nil {
		return errors.New("bootstrap_smtp_configuration_invalid")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		return errors.New("bootstrap_smtp_trust_unavailable")
	}
	smtp, err := deliverysmtp.New(deliverysmtp.Config{SenderName: in.SenderName, Host: in.SMTPHost, Port: in.SMTPPort, Username: in.SMTPUsername, Password: in.SMTPPassword, Sender: sender, TLS: &tls.Config{ServerName: in.SMTPHost, RootCAs: roots, MinVersion: tls.VersionTLS12}, Timeout: 20 * time.Second, MaxConcurrency: 1})
	if err != nil {
		return errors.New("bootstrap_smtp_configuration_invalid")
	}
	issuer, err := identitypersistence.NewEmailVerificationIssuer(pool)
	if err != nil {
		return errors.New("bootstrap_verification_unavailable")
	}
	service, err := identityapp.NewEmailVerificationIssueService(issuer, smtp, identityapp.SecureVerificationTokenGenerator{}, ids{}, clock{})
	if err != nil {
		return errors.New("bootstrap_verification_unavailable")
	}
	correlation := identityapp.ID(uuid.New())
	_, err = service.Issue(ctx, identityapp.EmailVerificationIssueInput{UserID: account.UserID, CorrelationID: correlation})
	if err != nil {
		return errors.New("bootstrap_verification_not_confirmed_do_not_blindly_retry")
	}
	// No raw token, credential, recipient, or message content is printed. The
	// correlation identifies the existing immutable verification-issuance audit.
	fmt.Printf("bootstrap_verification_smtp_accepted correlation_id=%s delivery_confirmed=false at=%s\n", uuid.UUID(correlation), time.Now().UTC().Format(time.RFC3339))
	return nil
}
func databaseConfig() (postgres.Config, error) {
	invalid := errors.New("bootstrap_database_configuration_not_sandbox")
	raw := os.Getenv("BIDORA_TEST_CONTROL_RUNTIME_DSN")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.Hostname() != "postgres" || u.Path != "/bidora_sandbox_control" || u.User.Username() != "bidora_sandbox_runtime" || u.Query().Get("sslmode") != "verify-full" {
		return postgres.Config{}, invalid
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port != 5432 {
		return postgres.Config{}, invalid
	}
	password, ok := u.User.Password()
	if !ok || password == "" {
		return postgres.Config{}, invalid
	}
	pem, err := os.ReadFile(os.Getenv("BIDORA_TEST_POSTGRES_CA_FILE"))
	if err != nil {
		return postgres.Config{}, invalid
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return postgres.Config{}, invalid
	}
	return postgres.Config{Host: u.Hostname(), Port: uint16(port), Database: strings.TrimPrefix(u.Path, "/"), User: u.User.Username(), Password: password, RootCAs: roots, MaxConnections: 1, ConnectTimeout: 3 * time.Second, OperationTimeout: 5 * time.Second, LockTimeout: time.Second, MaxLifetime: time.Minute, MaxIdleTime: time.Minute, HealthCheckPeriod: 30 * time.Second}, nil
}
