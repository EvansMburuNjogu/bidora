package smtpsecret

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

const pesaPalRefPrefix = "dbpesapal:v1:"

type PesaPalCredentials struct {
	Key    string `json:"consumer_key"`
	Secret string `json:"consumer_secret"`
}

func (PesaPalCredentials) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED PesaPal credentials]")
}
func ParsePesaPalCredentials(raw []byte) (c PesaPalCredentials, err error) {
	if json.Unmarshal(raw, &c) != nil {
		return c, errors.New("pesapal_credentials_invalid")
	}
	for _, s := range []string{c.Key, c.Secret} {
		if len(s) < 8 || len(s) > 512 || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\r\n\x00") {
			return PesaPalCredentials{}, errors.New("pesapal_credentials_invalid")
		}
	}
	return c, nil
}
func (v *Vault) PreparePesaPalSecrets(actor domain.ID, key string, consumerKey, secret *string) (string, []app.SealedIntegrationSecret, error) {
	if v == nil || actor.IsZero() || key == "" {
		return "", nil, errors.New("pesapal_secret_unavailable")
	}
	if consumerKey == nil && secret == nil {
		return "retain-active", nil, nil
	}
	if consumerKey == nil || secret == nil {
		return "", nil, errors.New("pesapal_secret_invalid")
	}
	raw, e := json.Marshal(PesaPalCredentials{Key: *consumerKey, Secret: *secret})
	if e != nil {
		return "", nil, e
	}
	defer wipe(raw)
	if _, e = ParsePesaPalCredentials(raw); e != nil {
		return "", nil, e
	}
	digest := sha256.Sum256(raw)
	id := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("bidora:integration-secret:"+uuid.UUID(actor).String()+":"+key+":PESAPAL_CREDENTIALS:"), digest[:]...))
	nonce, ciphertext, e := v.seal(id, "PESAPAL_CREDENTIALS", raw)
	if e != nil {
		return "", nil, e
	}
	return pesaPalRefPrefix + id.String(), []app.SealedIntegrationSecret{{ID: domain.ID(id), Kind: "PESAPAL_CREDENTIALS", KeyVersion: v.keyVersion, Nonce: nonce, Ciphertext: ciphertext}}, nil
}
