package identity_test

import (
	"bidora/backend/internal/identity/verification"
	"bytes"
	"fmt"
	"testing"
)

func TestShortEmailCodesBindPurposeReferenceAndSecret(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	ref := "00000000-0000-4000-8000-000000000001"
	token, err := verification.BoundCode(key, "verify", ref, "001234")
	if err != nil {
		t.Fatal(err)
	}
	if token.RevealForDelivery() != "001234" || token.ChallengeRef() != ref {
		t.Fatal("code lost leading zeros")
	}
	hash, err := verification.HashPresented(token.Authority())
	if err != nil || hash != token.Hash() {
		t.Fatal("short code does not reach existing strong-token consumer")
	}
	for _, tc := range []struct {
		purpose, ref, code string
		key                []byte
	}{{"recovery", ref, "001234", key}, {"verify", "00000000-0000-4000-8000-000000000002", "001234", key}, {"verify", ref, "001235", key}, {"verify", ref, "001234", bytes.Repeat([]byte{8}, 32)}} {
		other, err := verification.BoundCode(tc.key, tc.purpose, tc.ref, tc.code)
		if err != nil || other.Hash() == token.Hash() {
			t.Fatal("short code scope not isolated")
		}
	}
	if fmt.Sprint(token) != "[REDACTED verification token]" {
		t.Fatal("code leaked through formatting")
	}
	for _, code := range []string{"12345", "1234567", "abcdef", "１２３４５６"} {
		if _, err := verification.BoundCode(key, "verify", ref, code); err == nil {
			t.Fatal("invalid code accepted")
		}
	}
}
