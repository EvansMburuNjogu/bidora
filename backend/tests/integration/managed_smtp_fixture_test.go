package integration_test

import (
	"context"
	"encoding/pem"
	"errors"
	"net"
	"testing"
	"time"

	"bidora/backend/internal/pilot"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Seed only a designated integration database, never replace an existing active
// configuration. Delivery still uses the production active store, encrypted vault,
// TLS validation and notification recorder. Immutable fixture facts are retained.
func configureManagedSMTPFixture(t *testing.T, config *pilot.Config, fixture *pilotSMTPFixture, port int) {
	t.Helper()
	pool := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	config.SMTPVaultKey, config.SMTPVaultKeyVersion = randomFixtureBytes(t, 32), 1
	config.NotificationFingerprintKey, config.NotificationFingerprintKeyVersion = randomFixtureBytes(t, 32), 1
	config.EmailLoginHMACSecret = randomFixtureBytes(t, 32)
	vault, err := smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	actor, versionID := uuid.New(), uuid.New()
	roots := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.serverTLS.Certificates[0].Certificate[0]})
	credentialsRef, rootsRef, secrets, err := vault.PrepareSMTPSecrets(domain.ID(actor), uuid.NewString(), "smtp-user", "smtp-password", string(roots))
	if err != nil {
		t.Fatal(err)
	}
	if port == 0 {
		port = fixture.listener.Addr().(*net.TCPAddr).Port
	}
	smtp := &domain.SMTPConfiguration{Host: "127.0.0.1", Port: int32(port), CredentialSecretRef: credentialsRef, TrustRootsSecretRef: rootsRef, SenderName: "Bidora", SenderAddress: "noreply@example.invalid", ConnectTimeoutSeconds: 1, CommandTimeoutSeconds: 1, DeliveryTimeoutSeconds: 2, MaxConcurrency: 1}
	_, hash, err := domain.NewConfiguration(domain.SMTP, smtp, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		var active pgtype.UUID
		var sequence int64
		if e := db.QueryRow(ctx, `SELECT active_version_id,next_sequence FROM platform_settings.integration_heads WHERE setting_kind='SMTP' FOR UPDATE`).Scan(&active, &sequence); e != nil {
			return e
		}
		if active.Valid {
			return errors.New("managed SMTP fixture requires a disposable control database with no active SMTP configuration")
		}
		now := time.Now().UTC()
		if _, e := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,created_at,updated_at) VALUES($1,$2,$2,$3,$3,$3)`, actor, "smtp-fixture-"+actor.String()+"@example.invalid", now); e != nil {
			return e
		}
		for _, secret := range secrets {
			if _, e := db.Exec(ctx, `INSERT INTO platform_settings.smtp_secrets(id,secret_kind,key_version,nonce,ciphertext,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, uuid.UUID(secret.ID), secret.Kind, secret.KeyVersion, secret.Nonce, secret.Ciphertext, actor, now); e != nil {
				return e
			}
		}
		if _, e := db.Exec(ctx, `INSERT INTO platform_settings.integration_versions(id,setting_kind,sequence,state,version,configuration_hash,created_by,creation_reason,created_at,updated_at,activated_at) VALUES($1,'SMTP',$2,'ACTIVE',1,$3,$4,'Synthetic integration SMTP fixture',$5,$5,$5)`, versionID, sequence, hash[:], actor, now); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, `INSERT INTO platform_settings.smtp_versions(version_id,host,port,credential_secret_ref,trust_roots_secret_ref,sender_name,sender_address,connect_timeout_seconds,command_timeout_seconds,delivery_timeout_seconds,max_concurrency,starttls_required) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true)`, versionID, smtp.Host, smtp.Port, smtp.CredentialSecretRef, smtp.TrustRootsSecretRef, smtp.SenderName, smtp.SenderAddress, smtp.ConnectTimeoutSeconds, smtp.CommandTimeoutSeconds, smtp.DeliveryTimeoutSeconds, smtp.MaxConcurrency); e != nil {
			return e
		}
		_, e := db.Exec(ctx, `UPDATE platform_settings.integration_heads SET active_version_id=$1,next_sequence=next_sequence+1,generation=generation+1,updated_at=$2 WHERE setting_kind='SMTP'`, versionID, now)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	// Registered after livePool cleanup, so this runs while the pool is open.
	t.Cleanup(func() {
		err := pool.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			var active pgtype.UUID
			if e := db.QueryRow(ctx, `SELECT active_version_id FROM platform_settings.integration_heads WHERE setting_kind='SMTP' FOR UPDATE`).Scan(&active); e != nil {
				return e
			}
			if !active.Valid || active.Bytes != [16]byte(versionID) {
				return errors.New("managed SMTP fixture active version changed unexpectedly")
			}
			if _, e := db.Exec(ctx, `UPDATE platform_settings.integration_heads SET active_version_id=NULL,generation=generation+1,updated_at=CURRENT_TIMESTAMP WHERE setting_kind='SMTP'`); e != nil {
				return e
			}
			_, e := db.Exec(ctx, `UPDATE platform_settings.integration_versions SET state='SUPERSEDED',version=version+1,superseded_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, versionID)
			return e
		})
		if err != nil {
			t.Error(err)
		}
	})
}
