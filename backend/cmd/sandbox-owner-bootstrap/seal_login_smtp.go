package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/platformsettings/domain"
	qsql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Explicit sandbox deployment-operator setup for the dedicated login channel.
// Does not activate an SMTP settings revision or relax maker/checker policy.
func sealLoginSMTP(ctx context.Context, pool *postgres.Pool, actor app.ID, in input) error {
	invalid := errors.New("bootstrap_login_smtp_configuration_invalid")
	if in.SMTPHost != "smtp.zoho.com" || in.SMTPPort != 587 {
		return invalid
	}
	key, err := base64.RawStdEncoding.DecodeString(os.Getenv("BIDORA_LOCAL_SMTP_VAULT_KEY_B64"))
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(os.Getenv("BIDORA_LOCAL_SMTP_VAULT_KEY_B64"))
	}
	if err != nil || len(key) != 32 {
		return invalid
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	roots, err := os.ReadFile(os.Getenv("BIDORA_BOOTSTRAP_SMTP_TRUST_FILE"))
	if err != nil {
		return invalid
	}
	version, err := strconv.ParseInt(os.Getenv("BIDORA_LOCAL_SMTP_VAULT_KEY_VERSION"), 10, 32)
	if err != nil || version < 1 {
		return invalid
	}
	vault, err := smtpsecret.NewVault(pool, key, int32(version))
	if err != nil {
		return invalid
	}
	credentials, trust, envelopes, err := vault.PrepareSMTPSecrets(domain.ID(actor), "deployment-platform-login-smtp-v1", in.SMTPUsername, in.SMTPPassword, string(roots))
	if err != nil {
		return invalid
	}
	now := time.Now().UTC()
	correlation := uuid.New()
	err = pool.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		var eligible bool
		e := db.QueryRow(tc, `SELECT g.bootstrap_consumed AND g.bootstrap_owner_id=u.id AND u.account_type='PLATFORM_ADMIN' AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND EXISTS(SELECT 1 FROM platform_administration.role_assignments r WHERE r.user_id=u.id AND r.role_key='PLATFORM_OWNER' AND r.state='ACTIVE') FROM identity.users u CROSS JOIN platform_administration.governance_state g WHERE u.id=$1 FOR UPDATE OF u,g`, uuid.UUID(actor)).Scan(&eligible)
		if e != nil || !eligible {
			return errors.New("not eligible")
		}
		q := qsql.New(db)
		for _, sealed := range envelopes {
			if e = q.InsertSMTPSecret(tc, qsql.InsertSMTPSecretParams{ID: pgtype.UUID{Bytes: sealed.ID, Valid: true}, SecretKind: sealed.Kind, KeyVersion: sealed.KeyVersion, Nonce: sealed.Nonce, Ciphertext: sealed.Ciphertext, CreatedBy: pgtype.UUID{Bytes: actor, Valid: true}, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}); e != nil {
				return e
			}
		}
		_, e = db.Exec(tc, `INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES($1,NULL,'PlatformLoginEmailDeploymentConfigured','platform_administrator',$2,$3,'SUCCEEDED',$4,'{"source":"TRUSTED_SANDBOX_DEPLOYMENT","managed_smtp_activated":false,"secret_storage":"ENCRYPTED_VAULT"}'::jsonb,1,$3)`, uuid.New(), uuid.UUID(actor), now, correlation)
		if e != nil {
			return e
		}
		_, e = db.Exec(tc, `INSERT INTO bidora_control_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at) VALUES($1,'platform_administrator',$2,1,'platform.login_email.deployment_configured.v1',1,'INTERNAL','{"source":"TRUSTED_SANDBOX_DEPLOYMENT"}'::jsonb,$3,$3,$3)`, uuid.New(), uuid.UUID(actor), now)
		return e
	})
	for _, sealed := range envelopes {
		for i := range sealed.Ciphertext {
			sealed.Ciphertext[i] = 0
		}
	}
	if err != nil {
		return errors.New("bootstrap_login_smtp_seal_failed")
	}
	// References are opaque IDs, not plaintext credentials. Operator redirects this
	// output to the private deployment env file; nothing is exposed to browsers.
	fmt.Printf("BIDORA_PLATFORM_EMAIL_OTP=true\nBIDORA_PLATFORM_LOGIN_SMTP_HOST=%s\nBIDORA_PLATFORM_LOGIN_SMTP_PORT=%d\nBIDORA_PLATFORM_LOGIN_SMTP_SENDER=%s\nBIDORA_PLATFORM_LOGIN_SMTP_CREDENTIAL_REF=%s\nBIDORA_PLATFORM_LOGIN_SMTP_TRUST_REF=%s\n", in.SMTPHost, in.SMTPPort, in.SMTPUsername, credentials, trust)
	return nil
}
