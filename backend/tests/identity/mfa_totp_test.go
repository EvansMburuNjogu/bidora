package identity_test

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/mfa"
)

func TestMFATOTPRFC6238SHA1VectorsAndReplayWindow(t *testing.T) {
	secret := []byte("12345678901234567890")
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, vector := range vectors {
		now := time.Unix(vector.unix, 0).UTC()
		counter, ok := mfa.Verify(secret, vector.code, now, -1)
		if !ok || counter != vector.unix/30 {
			t.Fatalf("RFC 6238 vector rejected at %d", vector.unix)
		}
		if replayCounter, replayOK := mfa.Verify(secret, vector.code, now, counter); replayOK || replayCounter != 0 {
			t.Fatalf("accepted replay at %d", vector.unix)
		}
	}
	if _, ok := mfa.Verify(secret, "12345", time.Unix(59, 0), -1); ok {
		t.Fatal("accepted short TOTP")
	}
	if _, ok := mfa.Verify(secret, "12x456", time.Unix(59, 0), -1); ok {
		t.Fatal("accepted non-numeric TOTP")
	}
}

func TestMFAKeyRingVersionAndAADBinding(t *testing.T) {
	var key1, key2 [32]byte
	for i := range key1 {
		key1[i] = byte(i + 1)
		key2[i] = byte(255 - i)
	}
	oldRing, err := mfa.NewKeyRing(1, map[int][32]byte{1: key1})
	if err != nil {
		t.Fatal(err)
	}
	user, factor := [16]byte{15: 1}, [16]byte{15: 2}
	secret := []byte("12345678901234567890")
	ciphertext, nonce, version, err := oldRing.Encrypt(user, factor, secret)
	if err != nil || version != 1 || len(nonce) != 12 || string(ciphertext) == string(secret) {
		t.Fatal("old-key encryption drifted", version, len(nonce), err)
	}
	rotated, err := mfa.NewKeyRing(2, map[int][32]byte{1: key1, 2: key2})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := rotated.Decrypt(user, factor, ciphertext, nonce, version)
	if err != nil || string(plain) != string(secret) {
		t.Fatal("historical key version could not decrypt", err)
	}
	wrongUser, wrongFactor := user, factor
	wrongUser[15]++
	wrongFactor[15]++
	for name, candidate := range map[string]struct {
		user, factor [16]byte
		version      int
	}{
		"user AAD":    {wrongUser, factor, version},
		"factor AAD":  {user, wrongFactor, version},
		"unknown key": {user, factor, 99},
	} {
		if got, decryptErr := rotated.Decrypt(candidate.user, candidate.factor, ciphertext, nonce, candidate.version); decryptErr == nil || got != nil {
			t.Fatalf("%s binding accepted", name)
		}
	}
	newCiphertext, newNonce, newVersion, err := rotated.Encrypt(user, factor, secret)
	if err != nil || newVersion != 2 {
		t.Fatal("current key version not used", newVersion, err)
	}
	if _, err = oldRing.Decrypt(user, factor, newCiphertext, newNonce, newVersion); err == nil {
		t.Fatal("old-only ring decrypted new key version")
	}
}

func TestMFARecoveryCodesAreBoundedUniqueAndCaseInsensitive(t *testing.T) {
	codes, hashes, err := mfa.GenerateRecoveryCodes()
	if err != nil || len(codes) != 10 || len(hashes) != 10 {
		t.Fatal("recovery generation failed", err)
	}
	seen := map[string]struct{}{}
	for i, code := range codes {
		if len(code) != 16 || strings.Trim(code, "0123456789ABCDEFGHJKMNPQRSTVWXYZ") != "" {
			t.Fatal("invalid recovery-code shape")
		}
		if _, exists := seen[code]; exists {
			t.Fatal("duplicate recovery code")
		}
		seen[code] = struct{}{}
		if hashes[i] != mfa.HashRecovery(strings.ToLower(code)) || len(hex.EncodeToString(hashes[i][:])) != 64 {
			t.Fatal("recovery hash normalization drifted")
		}
	}
}
