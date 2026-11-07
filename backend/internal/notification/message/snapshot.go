// Package message encrypts secret-redacted communication previews.
package message

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"bidora/backend/internal/notification/domain"
)

type Content struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
}

func (Content) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED communication preview]") }

type Encrypted struct {
	KeyVersion        int32
	Nonce, Ciphertext []byte
}

func (Encrypted) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED encrypted communication]") }

type Cipher struct {
	aead           cipher.AEAD
	version        int32
	fingerprintKey []byte
}

func New(key []byte, version int32) (*Cipher, error) {
	if len(key) != 32 || version < 1 {
		return nil, errors.New("message_cipher_invalid")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("bidora:notification-message:encryption:v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead, version: version, fingerprintKey: append([]byte(nil), key...)}, nil
}

var links = regexp.MustCompile(`https?://[^\s<>]+`)
var tokens = regexp.MustCompile(`[A-Za-z0-9_-]{32,}`)
var codes = regexp.MustCompile(`[0-9]{6,}`)

func Redact(value string) string {
	value = links.ReplaceAllString(value, "[link redacted]")
	value = tokens.ReplaceAllString(value, "[secret redacted]")
	return codes.ReplaceAllString(value, "[code redacted]")
}
func bounded(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "\n[Preview truncated]"
}
func (c *Cipher) Seal(id domain.ID, v Content) (Encrypted, error) {
	if c == nil || id.IsZero() || v.Recipient == "" || len(v.Recipient) > 254 || v.Subject == "" || v.Text == "" {
		return Encrypted{}, errors.New("message_content_invalid")
	}
	v.Subject = bounded(Redact(v.Subject), 512)
	v.Text = bounded(Redact(v.Text), 6000)
	raw, err := json.Marshal(v)
	if err != nil {
		return Encrypted{}, err
	}
	defer clear(raw)
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return Encrypted{}, err
	}
	return Encrypted{KeyVersion: c.version, Nonce: nonce, Ciphertext: c.aead.Seal(nil, nonce, raw, append([]byte("bidora:notification-message:v1:"), id[:]...))}, nil
}
func (c *Cipher) Open(id domain.ID, v Encrypted) (Content, error) {
	if c == nil || id.IsZero() || v.KeyVersion != c.version || len(v.Nonce) != c.aead.NonceSize() || len(v.Ciphertext) > 65536 {
		return Content{}, errors.New("message_unavailable")
	}
	raw, err := c.aead.Open(nil, v.Nonce, v.Ciphertext, append([]byte("bidora:notification-message:v1:"), id[:]...))
	if err != nil {
		return Content{}, errors.New("message_unavailable")
	}
	defer clear(raw)
	var out Content
	if json.Unmarshal(raw, &out) != nil || out.Recipient == "" || out.Subject == "" || out.Text == "" || len(out.Recipient) > 254 || len(out.Subject) > 600 || len(out.Text) > 6100 {
		return Content{}, errors.New("message_unavailable")
	}
	return out, nil
}
func (c *Cipher) RecipientFingerprint(value string) []byte {
	if c == nil || value == "" {
		return nil
	}
	mac := hmac.New(sha256.New, c.fingerprintKey)
	mac.Write([]byte("bidora-notification-v1\x00recipient\x00"))
	mac.Write([]byte(strings.ToLower(value)))
	return mac.Sum(nil)
}
func (c *Cipher) KeyVersion() int32 {
	if c == nil {
		return 0
	}
	return c.version
}
func (c *Cipher) MatchesRecipient(value string, fingerprint []byte) bool {
	return len(fingerprint) == sha256.Size && hmac.Equal(c.RecipientFingerprint(value), fingerprint)
}
