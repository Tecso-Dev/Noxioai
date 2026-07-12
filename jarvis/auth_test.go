package main

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	password := "correct horse battery staple"

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
