package localbootstrap_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"bidora/backend/internal/localbootstrap"
)

func TestGeneratedCredentialSCRAMEncoding(t *testing.T) {
	credential, err := localbootstrap.GenerateCredential()
	if err != nil {
		t.Fatal("credential generation failed")
	}
	verifySCRAM(t, credential)
}

func verifySCRAM(t *testing.T, credential localbootstrap.Credential) {
	t.Helper()
	password := credential.Password()
	decoded, err := base64.RawURLEncoding.DecodeString(string(password))
	if err != nil || len(decoded) != 32 || len(password) != 43 {
		t.Fatal("password must encode 32 random bytes as canonical ASCII")
	}
	statement, err := credential.ActivationSQL("bidora_local_control_runtime")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(statement.Reveal(), "'")
	if len(parts) != 3 || parts[0] != `ALTER ROLE "bidora_local_control_runtime" PASSWORD ` || parts[2] != " LOGIN;" {
		t.Fatal("unsafe or unexpected activation statement shape")
	}
	verifier := strings.Split(parts[1], "$")
	if len(verifier) != 3 || verifier[0] != "SCRAM-SHA-256" {
		t.Fatal("unexpected verifier type")
	}
	params, keys := strings.Split(verifier[1], ":"), strings.Split(verifier[2], ":")
	if len(params) != 2 || params[0] != "4096" || len(keys) != 2 {
		t.Fatal("unexpected verifier parameters")
	}
	salt, err := base64.StdEncoding.DecodeString(params[1])
	if err != nil || len(salt) != 16 {
		t.Fatal("invalid salt")
	}
	stored, err := base64.StdEncoding.DecodeString(keys[0])
	if err != nil || len(stored) != 32 {
		t.Fatal("invalid stored key")
	}
	server, err := base64.StdEncoding.DecodeString(keys[1])
	if err != nil || len(server) != 32 {
		t.Fatal("invalid server key")
	}

	// Independently compute PBKDF2 block 1 by its HMAC/XOR definition rather
	// than calling production's crypto/pbkdf2 helper. This verifies encoding
	// and SCRAM key wiring, not successful PostgreSQL authentication.
	u := mac(password, append(append([]byte{}, salt...), 0, 0, 0, 1))
	salted := append([]byte{}, u...)
	for i := 1; i < 4096; i++ {
		u = mac(password, u)
		for j := range salted {
			salted[j] ^= u[j]
		}
	}
	wantStored := sha256.Sum256(mac(salted, []byte("Client Key")))
	if !bytes.Equal(stored, wantStored[:]) || !bytes.Equal(server, mac(salted, []byte("Server Key"))) {
		t.Fatal("SCRAM key derivation differs from independent computation")
	}
	password[0] ^= 1
	if bytes.Equal(password, credential.Password()) {
		t.Fatal("password accessor exposed mutable credential storage")
	}
}

func TestRestoreCredentialPreservesPasswordWithFreshVerifier(t *testing.T) {
	original, err := localbootstrap.GenerateCredential()
	if err != nil {
		t.Fatal(err)
	}
	stored := original.Password()
	restored, err := localbootstrap.RestoreCredential(stored)
	if err != nil || !bytes.Equal(original.Password(), restored.Password()) {
		t.Fatal("restore rotated the password")
	}
	verifySCRAM(t, restored)
	oldSQL, err := original.ActivationSQL("bidora_local_control_runtime")
	if err != nil {
		t.Fatal(err)
	}
	newSQL, err := restored.ActivationSQL("bidora_local_control_runtime")
	if err != nil || oldSQL.Reveal() == newSQL.Reveal() {
		t.Fatal("restore did not use a fresh salt")
	}
	stored[0] ^= 1
	if bytes.Equal(stored, restored.Password()) {
		t.Fatal("restore retained caller-owned mutable bytes")
	}
	if fmt.Sprintf("%#v", restored) != "[REDACTED]" {
		t.Fatal("restored credential leaked")
	}
}

func TestRestoreCredentialRejectsNoncanonicalPasswords(t *testing.T) {
	for _, input := range []string{"", "secret-canary", strings.Repeat("a", 42), strings.Repeat("a", 44), strings.Repeat("A", 42) + "B", strings.Repeat("A", 42) + "=", strings.Repeat("A", 42) + "\n", strings.Repeat("A", 42) + "\x00", strings.Repeat("A", 41) + "é", strings.Repeat("A", 42) + "+", strings.Repeat("A", 42) + "/"} {
		c, err := localbootstrap.RestoreCredential([]byte(input))
		if err == nil || len(c.Password()) != 0 || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsafe persisted password accepted or echoed")
		}
	}
}

func mac(key, message []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(message)
	return h.Sum(nil)
}

func TestCredentialsAreFreshAndRestrictedToFixedLoginRoles(t *testing.T) {
	seenPasswords, seenSalts := map[string]bool{}, map[string]bool{}
	for _, suffix := range []string{"control_migrator", "control_runtime", "tenant_a_migrator", "tenant_a_runtime", "tenant_b_migrator", "tenant_b_runtime"} {
		c, err := localbootstrap.GenerateCredential()
		if err != nil {
			t.Fatal("credential generation failed")
		}
		s, err := c.ActivationSQL("bidora_local_" + suffix)
		if err != nil {
			t.Fatal("fixed login role rejected")
		}
		password := string(c.Password())
		if seenPasswords[password] {
			t.Fatal("credential reused")
		}
		seenPasswords[password] = true
		salt := strings.Split(strings.Split(s.Reveal(), "'")[1], "$")[1]
		if seenSalts[salt] {
			t.Fatal("salt reused")
		}
		seenSalts[salt] = true
		if strings.Contains(s.Reveal(), password) {
			t.Fatal("cleartext password appeared in SQL")
		}
	}
	c, err := localbootstrap.GenerateCredential()
	if err != nil {
		t.Fatal("credential generation failed")
	}
	for _, role := range []string{"", "postgres", "bidora_local_bootstrap", "bidora_local_provisioner", "bidora_local_control_owner", "bidora_local_control_runtime ", `secret-canary"; DROP ROLE postgres; --`} {
		statement, err := c.ActivationSQL(role)
		if err == nil || statement.Reveal() != "" || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsafe role accepted or disclosed")
		}
	}
	var zero localbootstrap.Credential
	if statement, err := zero.ActivationSQL("bidora_local_control_runtime"); err == nil || statement.Reveal() != "" {
		t.Fatal("zero credential accepted")
	}
}

func TestCredentialsAndSQLRedactFormattingAndJSON(t *testing.T) {
	c, err := localbootstrap.GenerateCredential()
	if err != nil {
		t.Fatal("credential generation failed")
	}
	s, err := c.ActivationSQL("bidora_local_control_runtime")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{c, &c, s, &s, struct {
		C localbootstrap.Credential
		S localbootstrap.SecretSQL
	}{c, s}} {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			out := fmt.Sprintf(format, value)
			if strings.Contains(out, string(c.Password())) || strings.Contains(out, "SCRAM-SHA-256") || strings.Contains(out, "ALTER ROLE") {
				t.Fatal("sensitive formatting leak")
			}
		}
		out, err := json.Marshal(value)
		if err != nil || strings.Contains(string(out), string(c.Password())) || strings.Contains(string(out), "SCRAM-SHA-256") || !strings.Contains(string(out), "REDACTED") {
			t.Fatal("sensitive JSON leak")
		}
	}
	if c.String() != "[REDACTED]" || s.String() != "[REDACTED]" {
		t.Fatal("sensitive String method")
	}
}
