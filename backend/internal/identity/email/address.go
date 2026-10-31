// Package email implements the approved pilot mailbox identity policy. It does
// not send mail, prove address ownership or authorize authentication.
package email

import (
	"errors"
	"fmt"
	"strings"
)

// Address preserves the exact accepted display spelling and owns a deterministic
// lowercase lookup spelling. Formatting is redacted because email is personal data.
type Address struct{ display, lookup string }

func (a Address) Display() string             { return a.display }
func (a Address) Lookup() string              { return a.lookup }
func (a Address) IsZero() bool                { return a.display == "" || a.lookup == "" }
func (Address) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED email]") }

func Parse(input string) (Address, error) {
	if len(input) < 3 || len(input) > 254 {
		return Address{}, errors.New("email_invalid")
	}
	for i := 0; i < len(input); i++ {
		if input[i] < 0x21 || input[i] > 0x7e {
			return Address{}, errors.New("email_invalid")
		}
	}
	if strings.Count(input, "@") != 1 {
		return Address{}, errors.New("email_invalid")
	}
	local, domain, ok := strings.Cut(input, "@")
	if !ok || len(local) == 0 || len(local) > 64 || len(domain) == 0 || len(domain) > 253 {
		return Address{}, errors.New("email_invalid")
	}
	if local[0] == '.' || local[len(local)-1] == '.' || strings.Contains(local, "..") {
		return Address{}, errors.New("email_invalid")
	}
	for i := 0; i < len(local); i++ {
		if !localByte(local[i]) {
			return Address{}, errors.New("email_invalid")
		}
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || !alphaNumeric(label[0]) || !alphaNumeric(label[len(label)-1]) {
			return Address{}, errors.New("email_invalid")
		}
		for i := 0; i < len(label); i++ {
			if !alphaNumeric(label[i]) && label[i] != '-' {
				return Address{}, errors.New("email_invalid")
			}
		}
	}
	return Address{display: input, lookup: strings.ToLower(input)}, nil
}
func localByte(value byte) bool {
	if alphaNumeric(value) {
		return true
	}
	return strings.ContainsRune("!#$%&'*+-/=?^_`{|}~.", rune(value))
}
func alphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
