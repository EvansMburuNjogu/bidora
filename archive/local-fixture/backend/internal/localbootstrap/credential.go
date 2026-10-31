package localbootstrap

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// Credential is an in-memory synthetic fixture credential, not an application
// identity or secret vault. Formatting/JSON are redacted; explicit accessors
// return sensitive material for the future private-file/SQL boundary only.
// Go strings/copies do not provide guaranteed memory zeroization.
type Credential struct {
	password string
	verifier string
}

// GenerateCredential uses 32 random password bytes and a fresh 16-byte salt.
// The password is canonical ASCII base64url, avoiding SASLprep ambiguities.
// SCRAM-SHA-256 uses PostgreSQL 15's 4096-iteration default, not a production
// identity password policy. This function writes no files and starts no service.
func GenerateCredential() (Credential, error) {
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return Credential{}, fmt.Errorf("fixture password entropy unavailable: %w", err)
	}
	return deriveCredential(base64.RawURLEncoding.EncodeToString(passwordBytes))
}

// RestoreCredential reconstructs a verifier using an existing private fixture
// password and a new random salt. It does not rotate or persist the password,
// activate a role, or prove the current server state. Use only after the store's
// ownership checks; valid encoding alone is not proof of password provenance.
func RestoreCredential(password []byte) (Credential, error) {
	if len(password) != 43 {
		return Credential{}, errors.New("restoration requires a canonical fixture password")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(password))
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != string(password) {
		return Credential{}, errors.New("restoration requires a canonical fixture password")
	}
	return deriveCredential(string(password))
}

func deriveCredential(password string) (Credential, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return Credential{}, fmt.Errorf("fixture salt entropy unavailable: %w", err)
	}
	salted, err := pbkdf2.Key(sha256.New, password, salt, 4096, sha256.Size)
	if err != nil {
		return Credential{}, fmt.Errorf("fixture credential derivation failed: %w", err)
	}
	client := hmac.New(sha256.New, salted)
	client.Write([]byte("Client Key"))
	stored := sha256.Sum256(client.Sum(nil))
	server := hmac.New(sha256.New, salted)
	server.Write([]byte("Server Key"))
	encode := base64.StdEncoding.EncodeToString
	verifier := "SCRAM-SHA-256$4096:" + encode(salt) + "$" + encode(stored[:]) + ":" + encode(server.Sum(nil))
	return Credential{password: password, verifier: verifier}, nil
}

// Password returns a fresh copy for an explicitly owned private credential file.
// The caller must never log it or pass it in process arguments/environment.
func (c Credential) Password() []byte { return []byte(c.password) }

// ActivationSQL renders only the six fixed migrator/runtime identities. The
// verifier is constructed internally from base64/numeric components, never from
// caller SQL text, and contains no apostrophe. Execution still requires verified
// owned roles/grants and secret-safe logging; this method executes nothing.
func (c Credential) ActivationSQL(role string) (SecretSQL, error) {
	if !allowedCredentialRole(role) {
		return SecretSQL{}, errors.New("credential activation requires a fixed fixture login role")
	}
	if c.password == "" || c.verifier == "" {
		return SecretSQL{}, errors.New("credential activation requires a generated credential")
	}
	return SecretSQL{text: "ALTER ROLE " + quote(role) + " PASSWORD '" + c.verifier + "' LOGIN;"}, nil
}

func (Credential) Format(state fmt.State, verb rune) { fmt.Fprint(state, "[REDACTED]") }
func (Credential) String() string                    { return "[REDACTED]" }
func (Credential) MarshalJSON() ([]byte, error)      { return []byte(`"[REDACTED]"`), nil }

// SecretSQL protects ordinary formatting/JSON from leaking a SCRAM verifier.
// It is intentionally separate from the non-sensitive bootstrap plan.
type SecretSQL struct{ text string }

// Reveal explicitly crosses the sensitive execution boundary. Do not log,
// checkpoint, serialize or interpolate its result into an error.
func (s SecretSQL) Reveal() string { return s.text }

func (SecretSQL) Format(state fmt.State, verb rune) { fmt.Fprint(state, "[REDACTED]") }
func (SecretSQL) String() string                    { return "[REDACTED]" }
func (SecretSQL) MarshalJSON() ([]byte, error)      { return []byte(`"[REDACTED]"`), nil }
