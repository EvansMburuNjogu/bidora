package smtpsecret

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
)

// Reuse deployment-supplied AES-GCM key material, never the SMTP credential
// namespace. Google envelopes have distinct ID/version-bound associated data.
func (v *Vault) googleAAD(id domain.ID, version int32) []byte {
	return []byte(fmt.Sprintf("BIDORA:GOOGLE:CLIENT_SECRET:V1:%s:%d", uuid.UUID(id).String(), version))
}
func (v *Vault) SealGoogleClientSecret(id domain.ID, plain []byte) (app.GoogleCipher, error) {
	if v == nil || id.IsZero() || len(plain) < 8 || len(plain) > 1024 {
		return app.GoogleCipher{}, errors.New("google_secret_invalid")
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, e := io.ReadFull(rand.Reader, nonce); e != nil {
		return app.GoogleCipher{}, errors.New("google_secret_unavailable")
	}
	return app.GoogleCipher{ID: id, KeyVersion: v.keyVersion, Nonce: nonce, Ciphertext: v.aead.Seal(nil, nonce, plain, v.googleAAD(id, v.keyVersion))}, nil
}
func (v *Vault) OpenGoogleClientSecret(c app.GoogleCipher) ([]byte, error) {
	if v == nil || c.ID.IsZero() || c.KeyVersion != v.keyVersion || len(c.Nonce) != v.aead.NonceSize() || len(c.Ciphertext) < 17 || len(c.Ciphertext) > 1040 {
		return nil, errors.New("google_secret_unavailable")
	}
	plain, e := v.aead.Open(nil, c.Nonce, c.Ciphertext, v.googleAAD(c.ID, c.KeyVersion))
	if e != nil {
		return nil, errors.New("google_secret_unavailable")
	}
	return plain, nil
}
