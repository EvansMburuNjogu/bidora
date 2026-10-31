package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const SecretBytes = 20

var crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

type KeyRing struct {
	current int
	keys    map[int][32]byte
}

func NewKeyRing(current int, keys map[int][32]byte) (*KeyRing, error) {
	if current < 1 || len(keys) == 0 {
		return nil, errors.New("mfa_keyring_invalid")
	}
	if _, ok := keys[current]; !ok {
		return nil, errors.New("mfa_keyring_invalid")
	}
	copyKeys := make(map[int][32]byte, len(keys))
	for v, k := range keys {
		if v < 1 || k == ([32]byte{}) {
			return nil, errors.New("mfa_keyring_invalid")
		}
		copyKeys[v] = k
	}
	return &KeyRing{current, copyKeys}, nil
}
func (r *KeyRing) Encrypt(user, factor [16]byte, secret []byte) ([]byte, []byte, int, error) {
	if r == nil || len(secret) != SecretBytes {
		return nil, nil, 0, errors.New("mfa_encrypt_invalid")
	}
	key := r.keys[r.current]
	block, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, nil, 0, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, nil, 0, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, nil, 0, e
	}
	return g.Seal(nil, nonce, secret, aad(user, factor)), nonce, r.current, nil
}
func (r *KeyRing) Decrypt(user, factor [16]byte, ciphertext, nonce []byte, version int) ([]byte, error) {
	key, ok := r.keys[version]
	if r == nil || !ok {
		return nil, errors.New("mfa_decrypt_invalid")
	}
	block, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil || len(nonce) != g.NonceSize() {
		return nil, errors.New("mfa_decrypt_invalid")
	}
	plain, e := g.Open(nil, nonce, ciphertext, aad(user, factor))
	if e != nil || len(plain) != SecretBytes {
		return nil, errors.New("mfa_decrypt_invalid")
	}
	return plain, nil
}
func aad(user, factor [16]byte) []byte {
	v := make([]byte, 0, 42)
	v = append(v, []byte("BIDORA:TOTP:V1")...)
	v = append(v, user[:]...)
	return append(v, factor[:]...)
}
func GenerateSecret() ([]byte, error) {
	v := make([]byte, SecretBytes)
	_, e := rand.Read(v)
	return v, e
}
func SecretBase32(v []byte) (string, error) {
	if len(v) != SecretBytes {
		return "", errors.New("mfa_secret_invalid")
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(v), nil
}
func URI(issuer, account string, secret []byte) (string, error) {
	s, e := SecretBase32(secret)
	if e != nil || issuer == "" || account == "" || strings.ContainsAny(issuer+account, "\r\n") {
		return "", errors.New("mfa_uri_invalid")
	}
	q := url.Values{"secret": {s}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + q.Encode(), nil
}
func Verify(secret []byte, code string, now time.Time, last int64) (int64, bool) {
	if len(secret) != SecretBytes || len(code) != 6 || now.IsZero() {
		return 0, false
	}
	if _, e := strconv.Atoi(code); e != nil {
		return 0, false
	}
	center := now.UTC().Unix() / 30
	for _, counter := range []int64{center, center - 1, center + 1} {
		if counter <= last || counter < 0 {
			continue
		}
		expected := hotp(secret, uint64(counter))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return counter, true
		}
	}
	return 0, false
}
func hotp(secret []byte, counter uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], counter)
	h := hmac.New(sha1.New, secret)
	h.Write(b[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 15
	n := (uint32(sum[o])&127)<<24 | uint32(sum[o+1])<<16 | uint32(sum[o+2])<<8 | uint32(sum[o+3])
	return fmt.Sprintf("%06d", n%1000000)
}
func GenerateRecoveryCodes() ([]string, [][32]byte, error) {
	codes := make([]string, 10)
	hashes := make([][32]byte, 10)
	for i := range codes {
		raw := make([]byte, 10)
		if _, e := rand.Read(raw); e != nil {
			return nil, nil, e
		}
		var b strings.Builder
		var acc uint32
		bits := 0
		for _, v := range raw {
			acc = (acc << 8) | uint32(v)
			bits += 8
			for bits >= 5 {
				bits -= 5
				b.WriteByte(crockford[(acc>>bits)&31])
			}
		}
		codes[i] = b.String()
		hashes[i] = HashRecovery(codes[i])
	}
	return codes, hashes, nil
}
func HashRecovery(code string) [32]byte {
	return sha256.Sum256([]byte("BIDORA:MFA:RECOVERY:V1:" + strings.ToUpper(code)))
}
