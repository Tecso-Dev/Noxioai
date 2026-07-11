package main

import (
	"strings"
	"testing"
)

func TestParseDraftJSON(t *testing.T) {
	body := "Hi homfi team — I took a look at homfi.com and noticed the site isn't mobile-optimized, which loses property seekers browsing on phones. We rebuild real-estate sites with modern UX. Worth a quick call? Sobhan — NOXIOAI"
	li := "Hi — noticed homfi's website misses mobile buyers. We fix exactly that for agencies. Open to a short chat? Sobhan — NOXIOAI"
	good := "Here you go:\n```json\n{\"email\":{\"subject\":\"homfi's website is losing mobile buyers\",\"body\":\"" + body + "\"},\"linkedin\":\"" + li + "\"}\n```"

	d, err := parseDraftJSON(good, "homfi Warsaw")
	if err != nil {
		t.Fatalf("good draft rejected: %v", err)
	}
	if !strings.Contains(d.Email.Body, "homfi.com") {
		t.Errorf("body lost in parse: %q", d.Email.Body)
	}

	if _, err := parseDraftJSON(`{"email":{"subject":"","body":"`+body+`"},"linkedin":"`+li+`"}`, "homfi Warsaw"); err == nil {
		t.Error("missing subject must be rejected")
	}

	generic := `{"email":{"subject":"Grow your business","body":"` + strings.Repeat("We help companies grow online. ", 5) + `"},"linkedin":"` + strings.Repeat("Let us help you grow. ", 3) + `"}`
	if _, err := parseDraftJSON(generic, "homfi Warsaw"); err == nil {
		t.Error("draft that never names the company must be rejected (Principle 3)")
	}
}
