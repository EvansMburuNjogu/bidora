// Package password provides policy-explicit Argon2id PHC hashing. It owns no
// registration, recovery, session, email-verification or authorization policy.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	maxMemoryKiB     = 256 * 1024
	maxIterations    = 20
	maxParallelism   = 16
	maxSaltBytes     = 64
	maxKeyBytes      = 64
	maxPasswordBytes = 1024 * 1024
)

// Policy has deliberately no defaults. Deployment policy must supply calibrated
// values within this resource-safety envelope.
type Policy struct {
	MemoryKiB        uint32
	Iterations       uint32
	Parallelism      uint8
	SaltBytes        uint32
	KeyBytes         uint32
	MaxPasswordBytes uint32
}

// PilotPolicy is the explicitly approved 2026-10-18 profile. Callers still
// choose it explicitly; New never substitutes it for an invalid/zero policy.
func PilotPolicy() Policy {
	return Policy{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2, SaltBytes: 16, KeyBytes: 32, MaxPasswordBytes: 128}
}

// ValidateNew enforces only the approved new-password policy: valid UTF-8 and
// 12–128 bytes. It deliberately performs no trimming, normalization or
// composition checks. Verification of an existing PHC does not call this.
func ValidateNew(candidate []byte) error {
	if len(candidate) < 12 {
		return errors.New("password_too_short")
	}
	if len(candidate) > int(PilotPolicy().MaxPasswordBytes) {
		return errors.New("password_too_large")
	}
	if !utf8.Valid(candidate) {
		return errors.New("password_utf8_invalid")
	}
	return nil
}

func (p Policy) validate() error {
	if p.MemoryKiB < 8*uint32(p.Parallelism) || p.MemoryKiB > maxMemoryKiB || p.Iterations == 0 || p.Iterations > maxIterations || p.Parallelism == 0 || p.Parallelism > maxParallelism || p.SaltBytes < 16 || p.SaltBytes > maxSaltBytes || p.KeyBytes < 16 || p.KeyBytes > maxKeyBytes || p.MaxPasswordBytes == 0 || p.MaxPasswordBytes > maxPasswordBytes {
		return errors.New("password_policy_invalid")
	}
	return nil
}

type Hasher struct {
	policy Policy
	random io.Reader
}
type Verification struct{ Match, NeedsRehash bool }

func New(policy Policy) (*Hasher, error) { return NewWithRandom(policy, rand.Reader) }
func NewWithRandom(policy Policy, random io.Reader) (*Hasher, error) {
	if err := policy.validate(); err != nil || random == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("password_random_invalid")
	}
	return &Hasher{policy, random}, nil
}

func (h *Hasher) ValidateNew(candidate []byte) error {
	if h == nil {
		return errors.New("password_hasher_invalid")
	}
	return ValidateNew(candidate)
}

func (h *Hasher) Hash(password []byte) (string, error) {
	if h == nil || h.random == nil {
		return "", errors.New("password_hasher_invalid")
	}
	if len(password) > int(h.policy.MaxPasswordBytes) {
		return "", errors.New("password_too_large")
	}
	salt := make([]byte, h.policy.SaltBytes)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", errors.New("password_random_failed")
	}
	key := argon2.IDKey(password, salt, h.policy.Iterations, h.policy.MemoryKiB, h.policy.Parallelism, h.policy.KeyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.policy.MemoryKiB, h.policy.Iterations, h.policy.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// ValidatePHC confirms a dummy hash is well formed and uses this hasher's exact
// profile without performing expensive password work.
func (h *Hasher) ValidatePHC(phc string) error {
	if h == nil {
		return errors.New("password_hasher_invalid")
	}
	parsed, err := parse(phc)
	if err != nil {
		return err
	}
	if !sameHashPolicy(parsed.policy, h.policy) {
		return errors.New("password_phc_policy_mismatch")
	}
	return nil
}

func (h *Hasher) Verify(password []byte, phc string) (Verification, error) {
	if h == nil {
		return Verification{}, errors.New("password_hasher_invalid")
	}
	if len(password) > int(h.policy.MaxPasswordBytes) {
		return Verification{}, errors.New("password_too_large")
	}
	parsed, err := parse(phc)
	if err != nil {
		return Verification{}, err
	}
	key := argon2.IDKey(password, parsed.salt, parsed.policy.Iterations, parsed.policy.MemoryKiB, parsed.policy.Parallelism, uint32(len(parsed.key)))
	match := subtle.ConstantTimeCompare(key, parsed.key) == 1
	return Verification{Match: match, NeedsRehash: match && !sameHashPolicy(parsed.policy, h.policy)}, nil
}

type parsedPHC struct {
	policy    Policy
	salt, key []byte
}

func parse(phc string) (parsedPHC, error) {
	if len(phc) > 2048 {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	memory, ok := number(params[0], "m=")
	if !ok {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	iterations, ok := number(params[1], "t=")
	if !ok {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	parallel, ok := number(params[2], "p=")
	if !ok || parallel > 255 {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	if memory > maxMemoryKiB || iterations > maxIterations || uint32(parallel) > maxParallelism || len(salt) > int(maxSaltBytes) || len(key) > int(maxKeyBytes) || len(salt) < 16 || len(key) < 16 || memory < 8*parallel || iterations == 0 || parallel == 0 {
		return parsedPHC{}, errors.New("password_phc_invalid")
	}
	return parsedPHC{policy: Policy{MemoryKiB: uint32(memory), Iterations: uint32(iterations), Parallelism: uint8(parallel), SaltBytes: uint32(len(salt)), KeyBytes: uint32(len(key))}, salt: salt, key: key}, nil
}
func sameHashPolicy(a, b Policy) bool {
	return a.MemoryKiB == b.MemoryKiB && a.Iterations == b.Iterations && a.Parallelism == b.Parallelism && a.SaltBytes == b.SaltBytes && a.KeyBytes == b.KeyBytes
}
func number(value, prefix string) (uint64, bool) {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, false
	}
	n, err := strconv.ParseUint(value[len(prefix):], 10, 32)
	return n, err == nil
}
