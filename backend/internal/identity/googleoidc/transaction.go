package googleoidc

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const TransactionSecretBytes = 32

type Transaction struct {
	state, nonce, verifier, challenge, browserBinding string
}
type TransactionPurpose string

const (
	TransactionPurposeSignIn TransactionPurpose = "SIGN_IN"
	TransactionPurposeLink   TransactionPurpose = "LINK"
)

type TransactionMaterial struct {
	Nonce, Verifier      string
	BrowserBindingHash   [32]byte
	Purpose              TransactionPurpose
	PrincipalBindingHash [32]byte
	CredentialVersion    int64
}

func (Transaction) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC transaction]")
}
func (TransactionMaterial) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC transaction material]")
}
func (t Transaction) StateForAuthorization() string   { return t.state }
func (t Transaction) NonceForAuthorization() string   { return t.nonce }
func (t Transaction) PKCEChallenge() string           { return t.challenge }
func (t Transaction) BrowserBindingForCookie() string { return t.browserBinding }
func (t Transaction) MaterialForStore() TransactionMaterial {
	return TransactionMaterial{Nonce: t.nonce, Verifier: t.verifier, BrowserBindingHash: domainHash("bidora.google-oidc-browser.v1\x00", t.browserBinding), Purpose: TransactionPurposeSignIn}
}
func (t Transaction) MaterialForLink(userID, sessionID [16]byte, authEpoch, credentialVersion int64) (TransactionMaterial, error) {
	if t.IsZero() || userID == [16]byte{} || sessionID == [16]byte{} || authEpoch < 1 || credentialVersion < 1 {
		return TransactionMaterial{}, errors.New("google_oidc_link_binding_invalid")
	}
	material := t.MaterialForStore()
	material.Purpose = TransactionPurposeLink
	material.PrincipalBindingHash = linkBindingHash(userID, sessionID, authEpoch, credentialVersion)
	material.CredentialVersion = credentialVersion
	return material, nil
}
func (t Transaction) IsZero() bool {
	return t.state == "" || t.nonce == "" || t.verifier == "" || t.challenge == "" || t.browserBinding == ""
}
func (m TransactionMaterial) IsZero() bool {
	if !validTransactionValue(m.Nonce) || !validTransactionValue(m.Verifier) || m.BrowserBindingHash == [32]byte{} {
		return true
	}
	return m.Purpose != TransactionPurposeSignIn && m.Purpose != TransactionPurposeLink || m.Purpose == TransactionPurposeSignIn && (m.PrincipalBindingHash != [32]byte{} || m.CredentialVersion != 0) || m.Purpose == TransactionPurposeLink && (m.PrincipalBindingHash == [32]byte{} || m.CredentialVersion < 1)
}
func (m TransactionMaterial) IsSignIn() bool {
	return !m.IsZero() && m.Purpose == TransactionPurposeSignIn
}
func (m TransactionMaterial) MatchesLink(userID, sessionID [16]byte, authEpoch int64) bool {
	if m.IsZero() || m.Purpose != TransactionPurposeLink || userID == [16]byte{} || sessionID == [16]byte{} || authEpoch < 1 {
		return false
	}
	expected := linkBindingHash(userID, sessionID, authEpoch, m.CredentialVersion)
	return subtle.ConstantTimeCompare(expected[:], m.PrincipalBindingHash[:]) == 1
}
func GenerateTransaction() (Transaction, error) { return GenerateTransactionWithRandom(rand.Reader) }
func GenerateTransactionWithRandom(random io.Reader) (Transaction, error) {
	if random == nil {
		return Transaction{}, errors.New("google_oidc_transaction_random_invalid")
	}
	values := make([]string, 4)
	for index := range values {
		raw := make([]byte, TransactionSecretBytes)
		if _, err := io.ReadFull(random, raw); err != nil {
			return Transaction{}, errors.New("google_oidc_transaction_random_failed")
		}
		values[index] = base64.RawURLEncoding.EncodeToString(raw)
	}
	challengeHash := sha256.Sum256([]byte(values[2]))
	return Transaction{state: values[0], nonce: values[1], verifier: values[2], challenge: base64.RawURLEncoding.EncodeToString(challengeHash[:]), browserBinding: values[3]}, nil
}
func HashPresentedState(encoded string) ([32]byte, error) {
	if !validTransactionValue(encoded) {
		return [32]byte{}, errors.New("google_oidc_state_invalid")
	}
	return domainHash("bidora.google-oidc-state.v1\x00", encoded), nil
}
func HashPresentedBrowserBinding(encoded string) ([32]byte, error) {
	if !validTransactionValue(encoded) {
		return [32]byte{}, errors.New("google_oidc_browser_binding_invalid")
	}
	return domainHash("bidora.google-oidc-browser.v1\x00", encoded), nil
}
func MatchesBrowserBinding(encoded string, expected [32]byte) (bool, error) {
	actual, err := HashPresentedBrowserBinding(encoded)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual[:], expected[:]) == 1, nil
}
func ParseTransactionMaterial(nonce, verifier string, browserBindingHash [32]byte, purpose TransactionPurpose, principalBindingHash [32]byte, credentialVersion int64) (TransactionMaterial, error) {
	material := TransactionMaterial{Nonce: nonce, Verifier: verifier, BrowserBindingHash: browserBindingHash, Purpose: purpose, PrincipalBindingHash: principalBindingHash, CredentialVersion: credentialVersion}
	if material.IsZero() {
		return TransactionMaterial{}, errors.New("google_oidc_transaction_material_invalid")
	}
	return material, nil
}
func validTransactionValue(value string) bool {
	if len(value) != base64.RawURLEncoding.EncodedLen(TransactionSecretBytes) {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(raw) == TransactionSecretBytes
}
func linkBindingHash(userID, sessionID [16]byte, authEpoch, credentialVersion int64) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("bidora.google-oidc-link.v1\x00"))
	_, _ = hash.Write(userID[:])
	_, _ = hash.Write(sessionID[:])
	var epoch [8]byte
	binary.BigEndian.PutUint64(epoch[:], uint64(authEpoch))
	_, _ = hash.Write(epoch[:])
	binary.BigEndian.PutUint64(epoch[:], uint64(credentialVersion))
	_, _ = hash.Write(epoch[:])
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
func domainHash(domain, encoded string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write([]byte(encoded))
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
