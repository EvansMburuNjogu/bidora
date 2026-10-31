package smtpsecret

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

// PrepareSMTPSecrets leaves persistence to the existing settings command UoW,
// making ciphertext, version, receipt, audit and outbox atomic and replay-safe.
func (v *Vault) PrepareSMTPSecrets(actor domain.ID, key, username, password, roots string) (string, string, []app.SealedIntegrationSecret, error) {
	if v == nil || actor.IsZero() || key == "" {
		return "", "", nil, errors.New("smtp_secret_unavailable")
	}
	raw, e := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{username, password})
	if e != nil {
		return "", "", nil, e
	}
	defer wipe(raw)
	if _, e = ParseCredentials(raw); e != nil {
		return "", "", nil, e
	}
	rootBytes := []byte(roots)
	defer wipe(rootBytes)
	if _, e = ParseTrustRoots(rootBytes); e != nil {
		return "", "", nil, e
	}
	var out []app.SealedIntegrationSecret
	refs := make([]string, 0, 2)
	for _, item := range []struct {
		kind string
		raw  []byte
	}{{"CREDENTIALS", raw}, {"TRUST_ROOTS", rootBytes}} {
		digest := sha256.Sum256(item.raw)
		id := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("bidora:integration-secret:"+uuid.UUID(actor).String()+":"+key+":"+item.kind+":"), digest[:]...))
		nonce, ciphertext, e := v.seal(id, item.kind, item.raw)
		if e != nil {
			return "", "", nil, e
		}
		out = append(out, app.SealedIntegrationSecret{ID: domain.ID(id), Kind: item.kind, KeyVersion: v.keyVersion, Nonce: nonce, Ciphertext: ciphertext})
		refs = append(refs, vaultRefPrefix+id.String())
	}
	return refs[0], refs[1], out, nil
}
