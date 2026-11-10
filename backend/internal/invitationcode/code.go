// Package invitationcode keeps the link context secret while presenting only
// six numeric digits. The context and digits together retain over 128 bits of
// entropy; a bare six-digit value is never an invitation credential.
package invitationcode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
)

var digits = regexp.MustCompile(`^[0-9]{6}$`)

func Generate(random io.Reader, purpose string) (string, [32]byte, error) {
	if random == nil {
		return "", [32]byte{}, errors.New("invitation_random_invalid")
	}
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return "", [32]byte{}, err
	}
	number, err := rand.Int(random, big.NewInt(1000000))
	if err != nil {
		return "", [32]byte{}, err
	}
	wire := base64.RawURLEncoding.EncodeToString(nonce) + "." + fmt.Sprintf("%06d", number.Int64())
	hash, err := Hash(purpose, wire)
	return wire, hash, err
}
func Split(wire string) (string, string, bool) {
	nonce, code, ok := strings.Cut(wire, ".")
	if !ok || len(nonce) != 22 || !digits.MatchString(code) {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(nonce)
	if err != nil || len(raw) != 16 {
		return "", "", false
	}
	return nonce, code, true
}
func Hash(purpose, wire string) ([32]byte, error) {
	if purpose != "platform" && purpose != "tenant" {
		return [32]byte{}, errors.New("invitation_purpose_invalid")
	}
	if _, _, ok := Split(wire); !ok {
		return [32]byte{}, errors.New("invitation_code_invalid")
	}
	return sha256.Sum256([]byte("bidora.invitation-code.v1\x00" + purpose + "\x00" + wire)), nil
}
func Display(wire string) string {
	if _, code, ok := Split(wire); ok {
		return code
	}
	return wire
}
