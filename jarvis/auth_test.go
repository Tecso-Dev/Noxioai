package main

import (
	"encoding/base64"
	"fmt"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	password := "a long private passphrase"

	first, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	second, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}

	if !verifyPassword(first, password) {
		t.Fatal("verifyPassword accepted neither the original password nor its hash")
	}
	if verifyPassword(first, "wrong password") {
		t.Fatal("verifyPassword accepted a wrong password")
	}
	if first == second {
		t.Fatal("hashPassword produced identical hashes for the same password")
	}
}

func TestLegacyArgon2HashStillVerifiesAndNeedsRehash(t *testing.T) {
	password := "a sufficiently long password"
	salt := []byte("0123456789abcdef")
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=65536,t=1,p=4$%s$%s",
		argon2.Version,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	if !verifyPassword(encoded, password) {
		t.Fatal("legacy Argon2id hash no longer verifies")
	}
	if !passwordNeedsRehash(encoded) {
		t.Fatal("legacy Argon2id parameters should be upgraded after login")
	}
}

func TestMalformedOrExcessiveArgon2ParametersAreRejected(t *testing.T) {
	for _, encoded := range []string{
		"not-a-hash",
		"$argon2id$v=19$m=999999,t=1,p=1$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
		"$argon2id$v=19$m=65536,t=99,p=1$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
	} {
		if verifyPassword(encoded, "a sufficiently long password") {
			t.Fatalf("verifyPassword accepted unsafe encoding %q", encoded)
		}
	}
}

func TestUnicodePasswordsUseCanonicalNormalization(t *testing.T) {
	decomposed := "Cafe\u0301 has a long passphrase"
	precomposed := "Café has a long passphrase"
	hash, err := hashPassword(decomposed)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !verifyPassword(hash, precomposed) {
		t.Fatal("canonically equivalent Unicode password did not verify")
	}
}
