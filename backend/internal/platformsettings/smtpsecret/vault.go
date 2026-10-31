package smtpsecret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"bidora/backend/internal/platformsettings/domain"
	settingssql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const vaultRefPrefix = "dbsmtp:v1:"

type Vault struct {
	runner     *postgres.Pool
	aead       cipher.AEAD
	keyVersion int32
}

func NewVault(runner *postgres.Pool, key []byte, keyVersion int32) (*Vault, error) {
	if runner == nil || len(key) != 32 || keyVersion < 1 {
		return nil, errors.New("smtp_secret_vault_invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("smtp_secret_vault_invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("smtp_secret_vault_invalid")
	}
	return &Vault{runner: runner, aead: aead, keyVersion: keyVersion}, nil
}

func (v *Vault) StoreSMTPSecrets(ctx context.Context, actor domain.ID, username, password, trustRoots string) (string, string, error) {
	if ctx == nil || v == nil || v.runner == nil || actor.IsZero() {
		return "", "", errors.New("smtp_secret_vault_unavailable")
	}
	credentials, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{username, password})
	if err != nil {
		return "", "", errors.New("smtp_secret_vault_invalid")
	}
	defer wipe(credentials)
	if _, err = ParseCredentials(credentials); err != nil {
		return "", "", err
	}
	roots := []byte(trustRoots)
	defer wipe(roots)
	if _, err = ParseTrustRoots(roots); err != nil {
		return "", "", err
	}
	credentialID, rootID := uuid.New(), uuid.New()
	credentialNonce, credentialCipher, err := v.seal(credentialID, "CREDENTIALS", credentials)
	if err != nil {
		return "", "", err
	}
	rootNonce, rootCipher, err := v.seal(rootID, "TRUST_ROOTS", roots)
	if err != nil {
		return "", "", err
	}
	at := time.Now().UTC()
	err = v.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := settingssql.New(db)
		createdBy := pgtype.UUID{Bytes: [16]byte(actor), Valid: true}
		createdAt := pgtype.Timestamptz{Time: at, Valid: true}
		if e := q.InsertSMTPSecret(txCtx, settingssql.InsertSMTPSecretParams{ID: pgtype.UUID{Bytes: credentialID, Valid: true}, SecretKind: "CREDENTIALS", KeyVersion: v.keyVersion, Nonce: credentialNonce, Ciphertext: credentialCipher, CreatedBy: createdBy, CreatedAt: createdAt}); e != nil {
			return e
		}
		return q.InsertSMTPSecret(txCtx, settingssql.InsertSMTPSecretParams{ID: pgtype.UUID{Bytes: rootID, Valid: true}, SecretKind: "TRUST_ROOTS", KeyVersion: v.keyVersion, Nonce: rootNonce, Ciphertext: rootCipher, CreatedBy: createdBy, CreatedAt: createdAt})
	})
	wipe(credentialCipher)
	wipe(rootCipher)
	if err != nil {
		return "", "", errors.New("smtp_secret_vault_unavailable")
	}
	return vaultRefPrefix + credentialID.String(), vaultRefPrefix + rootID.String(), nil
}

func (v *Vault) WithSecret(ctx context.Context, ref string, use func([]byte) error) error {
	if ctx == nil || v == nil || v.runner == nil || use == nil || (!strings.HasPrefix(ref, vaultRefPrefix) && !strings.HasPrefix(ref, openAIRefPrefix) && !strings.HasPrefix(ref, pesaPalRefPrefix)) {
		return errors.New("smtp_secret_unavailable")
	}
	prefix := vaultRefPrefix
	if strings.HasPrefix(ref, openAIRefPrefix) {
		prefix = openAIRefPrefix
	}
	if strings.HasPrefix(ref, pesaPalRefPrefix) {
		prefix = pesaPalRefPrefix
	}
	id, err := uuid.Parse(strings.TrimPrefix(ref, prefix))
	if err != nil || id == uuid.Nil {
		return errors.New("smtp_secret_unavailable")
	}
	var kind string
	var version int32
	var nonce, encrypted []byte
	err = v.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, queryErr := settingssql.New(db).GetSMTPSecret(txCtx, pgtype.UUID{Bytes: id, Valid: true})
		if queryErr != nil {
			return queryErr
		}
		kind, version, nonce, encrypted = row.SecretKind, row.KeyVersion, row.Nonce, row.Ciphertext
		return nil
	})
	if err != nil || version != v.keyVersion || (prefix == openAIRefPrefix) != (kind == "OPENAI_API_KEY") || (prefix == pesaPalRefPrefix) != (kind == "PESAPAL_CREDENTIALS") || len(nonce) != v.aead.NonceSize() {
		return errors.New("smtp_secret_unavailable")
	}
	plain, err := v.aead.Open(nil, nonce, encrypted, v.aad(id, kind))
	wipe(encrypted)
	if err != nil {
		return errors.New("smtp_secret_unavailable")
	}
	defer wipe(plain)
	return use(plain)
}

func (v *Vault) seal(id uuid.UUID, kind string, plain []byte) ([]byte, []byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, errors.New("smtp_secret_vault_unavailable")
	}
	return nonce, v.aead.Seal(nil, nonce, plain, v.aad(id, kind)), nil
}
func (v *Vault) aad(id uuid.UUID, kind string) []byte {
	return []byte(fmt.Sprintf("BIDORA:SMTP:SECRET:V1:%s:%s:%d", id.String(), kind, v.keyVersion))
}
func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
