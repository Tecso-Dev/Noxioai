package main

import "testing"

func TestSplitDraft(t *testing.T) {
	s, b := splitDraft("Subject: Hello there\n\nBody line one.\nLine two.")
	if s != "Hello there" || b != "Body line one.\nLine two." {
		t.Fatalf("got subject=%q body=%q", s, b)
	}
	if s, b = splitDraft("just a body, no subject"); s != "" || b != "just a body, no subject" {
		t.Fatalf("subjectless draft mishandled: %q %q", s, b)
	}
}
