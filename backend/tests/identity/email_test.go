package identity_test

import (
	"fmt"
	"strings"
	"testing"

	"bidora/backend/internal/identity/email"
)

func TestPilotEmailPreservesDisplayAndOwnsLowercaseLookup(t *testing.T) {
	input := "First.Last+Tag@Example-Corp.COM"
	address, err := email.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if address.Display() != input || address.Lookup() != "first.last+tag@example-corp.com" || address.IsZero() {
		t.Fatal("email representation drifted")
	}
	if strings.Contains(fmt.Sprintf("%v", address), "First") || strings.Contains(fmt.Sprintf("%#v", address), "Example") {
		t.Fatal("email formatting leaked personal data")
	}
	second, err := email.Parse("first.last+tag@example-corp.com")
	if err != nil || second.Lookup() != address.Lookup() {
		t.Fatal("case-equivalent address did not share lookup identity")
	}
	if strings.Contains(address.Lookup(), "firstlast") || !strings.Contains(address.Lookup(), "+tag") {
		t.Fatal("provider alias rewriting occurred")
	}
}

func TestPilotEmailAcceptsBoundariesAndDotAtomCharacters(t *testing.T) {
	valid := []string{"a@b", "AZaz09!#$%&'*+-/=?^_`{|}~.x@example.com", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)}
	for _, value := range valid {
		if address, err := email.Parse(value); err != nil || address.Display() != value {
			t.Fatal("valid address rejected", len(value), err)
		}
	}
}

func TestPilotEmailRejectsAmbiguousAndNonASCIIForms(t *testing.T) {
	invalid := []string{"", "a@", "@b", "a@@b", " a@b", "a@b ", "a b@c", "a\tb@c", "a\nb@c", "José@example.com", "Name <a@example.com>", "a(comment)@example.com", "\"a b\"@example.com", ".a@example.com", "a.@example.com", "a..b@example.com", "a()@example.com", "a@-example.com", "a@example-.com", "a@example..com", "a@exam_ple.com", strings.Repeat("a", 65) + "@example.com", "a@" + strings.Repeat("b", 64) + ".com", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 62)}
	for i, value := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			address, err := email.Parse(value)
			if err == nil || err.Error() != "email_invalid" || !address.IsZero() || value != "" && strings.Contains(fmt.Sprintf("%#v", err), value) {
				t.Fatal("invalid address accepted/leaked", address, err)
			}
		})
	}
	var zero email.Address
	if !zero.IsZero() || zero.Display() != "" || zero.Lookup() != "" {
		t.Fatal("zero email became valid")
	}
}
