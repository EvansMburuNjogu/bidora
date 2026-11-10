package identity_test

import (
	tenant "bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/invitationcode"
	platform "bidora/backend/internal/platformadmin/invitation"
	"regexp"
	"strings"
	"testing"
)

func TestInvitationCodesAlwaysSixDigitsAndKeepSecretContext(t *testing.T) {
	for _, kind := range []string{"platform", "tenant"} {
		t.Run(kind, func(t *testing.T) {
			var wire string
			var hash [32]byte
			if kind == "platform" {
				token, err := platform.Generate()
				if err != nil {
					t.Fatal(err)
				}
				wire, hash = token.RevealForDelivery(), token.Hash()
			} else {
				token, err := tenant.Generate()
				if err != nil {
					t.Fatal(err)
				}
				wire, hash = token.RevealForDelivery(), token.Hash()
			}
			nonce, code, ok := invitationcode.Split(wire)
			if !ok || len(nonce) != 22 || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
				t.Fatal("invitation code is not six digits")
			}
			var got [32]byte
			var err error
			if kind == "platform" {
				got, err = platform.HashPresented(wire)
			} else {
				got, err = tenant.HashPresented(wire)
			}
			if err != nil || got != hash {
				t.Fatal("scoped code cannot be redeemed")
			}
			if _, err = platform.HashPresented(code); err == nil {
				t.Fatal("bare six digits granted authority")
			}
			other, _ := invitationcode.Hash(map[string]string{"platform": "tenant", "tenant": "platform"}[kind], wire)
			if other == hash {
				t.Fatal("cross-purpose invitation accepted")
			}
			changed := nonce + "." + map[bool]string{true: "999999", false: "000000"}[code == "000000"]
			if changedHash, _ := invitationcode.Hash(kind, changed); changedHash == hash {
				t.Fatal("wrong code accepted")
			}
			wrongNonce := strings.Repeat("A", 22) + "." + code
			if wrongNonce == wire {
				wrongNonce = strings.Repeat("Q", 21) + "A." + code
			}
			if changedHash, _ := invitationcode.Hash(kind, wrongNonce); changedHash == hash {
				t.Fatal("wrong link context accepted")
			}
		})
	}
}
