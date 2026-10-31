package smtpsecret

import (
	"crypto/sha256"
	"errors"
	"strings"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/google/uuid"
)

const openAIRefPrefix = "dbopenai:v1:"

// PrepareOpenAISecrets only encrypts. The application transaction inserts the
// envelopes with version, audit, outbox and idempotency; retries do not orphan secrets.
func (v *Vault) PrepareOpenAISecrets(actor domain.ID, idempotency string, key, roots *string) (string, string, []app.SealedIntegrationSecret, error) {
	if v == nil || actor.IsZero() || idempotency == "" {
		return "", "", nil, errors.New("openai_secret_unavailable")
	}
	keyRef, rootsRef := "retain-active", "retain-active"
	var out []app.SealedIntegrationSecret
	for _, item := range []struct {
		kind  string
		value *string
	}{{"OPENAI_API_KEY", key}, {"TRUST_ROOTS", roots}} {
		if item.value == nil {
			continue
		}
		raw := []byte(*item.value)
		if item.kind == "OPENAI_API_KEY" {
			if len(raw) < 8 || len(raw) > 1024 || strings.TrimSpace(*item.value) != *item.value || strings.ContainsAny(*item.value, "\r\n\x00") {
				wipe(raw)
				return "", "", nil, errors.New("openai_secret_invalid")
			}
		} else {
			if _, e := ParseTrustRoots(raw); e != nil {
				wipe(raw)
				return "", "", nil, errors.New("openai_roots_invalid")
			}
		}
		digest := sha256.Sum256(raw)
		id := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("bidora:integration-secret:"+uuid.UUID(actor).String()+":"+idempotency+":"+item.kind+":"), digest[:]...))
		nonce, ciphertext, e := v.seal(id, item.kind, raw)
		wipe(raw)
		if e != nil {
			return "", "", nil, e
		}
		out = append(out, app.SealedIntegrationSecret{ID: domain.ID(id), Kind: item.kind, KeyVersion: v.keyVersion, Nonce: nonce, Ciphertext: ciphertext})
		if item.kind == "OPENAI_API_KEY" {
			keyRef = openAIRefPrefix + id.String()
		} else {
			rootsRef = vaultRefPrefix + id.String()
		}
	}
	return keyRef, rootsRef, out, nil
}
