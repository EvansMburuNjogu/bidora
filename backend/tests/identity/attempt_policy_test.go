package identity_test

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/attempt"
)

func TestApprovedAttemptPolicyIsExact(t *testing.T) {
	p := attempt.PilotPolicy()
	if p.IdentityFailureLimit != 5 || p.IdentityWindow != 15*time.Minute || p.IdentityDeny != 15*time.Minute || p.NetworkMinuteLimit != 30 || p.NetworkMinuteWindow != time.Minute || p.NetworkWindowLimit != 100 || p.NetworkWindow != 15*time.Minute || p.NetworkDeny != 15*time.Minute {
		t.Fatal("attempt policy drifted", p)
	}
}

func TestAttemptScopesAreHMACDerivedDomainSeparatedAndCanonical(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	deriver, err := attempt.NewDeriver(secret)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := deriver.Identity("user@example.com")
	if err != nil || identity.IsZero() {
		t.Fatal("identity scope failed", err)
	}
	network, err := deriver.Network(netip.MustParseAddr("192.0.2.1"))
	if err != nil || network.IsZero() {
		t.Fatal("network scope failed", err)
	}
	if identity == network {
		t.Fatal("attempt scopes lack domain separation")
	}
	registrationIdentity, err := deriver.RegistrationIdentity("user@example.com")
	if err != nil || registrationIdentity.IsZero() || registrationIdentity == identity {
		t.Fatal("registration identity scope lacks domain separation", err)
	}
	registrationNetwork, err := deriver.RegistrationNetwork(netip.MustParseAddr("192.0.2.1"))
	if err != nil || registrationNetwork.IsZero() || registrationNetwork == network || registrationNetwork == registrationIdentity {
		t.Fatal("registration network scope lacks domain separation", err)
	}
	var user, target [16]byte
	user[15], target[15] = 1, 2
	mfaLogin, err := deriver.MFA("login", user, target)
	if err != nil || mfaLogin.IsZero() {
		t.Fatal("MFA scope derivation failed", err)
	}
	mfaStepUp, err := deriver.MFA("step-up", user, target)
	if err != nil || mfaStepUp.IsZero() || mfaStepUp == mfaLogin || mfaStepUp == identity || mfaStepUp == network {
		t.Fatal("MFA scope lacks purpose/domain separation", err)
	}
	otherTarget := target
	otherTarget[15] = 3
	mfaOther, _ := deriver.MFA("login", user, otherTarget)
	if mfaOther == mfaLogin {
		t.Fatal("MFA target binding missing")
	}
	mapped, _ := deriver.Network(netip.MustParseAddr("::ffff:192.0.2.1"))
	if mapped != network {
		t.Fatal("mapped client address produced different scope")
	}
	secret[0] ^= 0xff
	again, _ := deriver.Identity("user@example.com")
	if again != identity {
		t.Fatal("caller mutated HMAC secret")
	}
	other, _ := deriver.Identity("other@example.com")
	if other == identity {
		t.Fatal("different identities shared scope")
	}
	if strings.Contains(fmt.Sprintf("%#v", identity), "user@example.com") {
		t.Fatal("attempt key formatting leaked identity")
	}
	if !(attempt.Scopes{Identity: identity, Network: network}).Valid() {
		t.Fatal("valid scopes rejected")
	}
}

func TestAttemptDeriverRejectsMissingInputs(t *testing.T) {
	if d, err := attempt.NewDeriver(make([]byte, 31)); err == nil || err.Error() != "attempt_hmac_secret_invalid" || d != nil {
		t.Fatal("short HMAC secret accepted")
	}
	deriver, _ := attempt.NewDeriver(make([]byte, 32))
	if key, err := deriver.Identity(""); err == nil || err.Error() != "attempt_identity_scope_invalid" || !key.IsZero() {
		t.Fatal("empty identity accepted")
	}
	if key, err := deriver.Network(netip.Addr{}); err == nil || err.Error() != "attempt_network_scope_invalid" || !key.IsZero() {
		t.Fatal("invalid network accepted")
	}
	if key, err := deriver.RegistrationIdentity(""); err == nil || !key.IsZero() {
		t.Fatal("empty registration identity accepted")
	}
	if key, err := deriver.RegistrationNetwork(netip.Addr{}); err == nil || !key.IsZero() {
		t.Fatal("invalid registration network accepted")
	}
	var zero *attempt.Deriver
	if _, err := zero.Identity("user@example.com"); err == nil {
		t.Fatal("nil deriver accepted")
	}
	if (attempt.Scopes{}).Valid() {
		t.Fatal("zero scopes accepted")
	}
}
