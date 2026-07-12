package main

import "testing"

const ddgFixture = `
<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwarsaw%2Dhomes.pl%2F&amp;rut=abc">Warsaw Homes</a>
<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.facebook.com%2Fsomeagency&amp;rut=def">FB junk</a>
<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwarsaw-homes.pl%2Fabout&amp;rut=ghi">Duplicate host</a>
<a rel="nofollow" class="result__a" href="https://plainlink.example.com/page">Plain link</a>`

func TestParseSearchResults(t *testing.T) {
	got := parseSearchResults(ddgFixture)
	if len(got) != 2 {
		t.Fatalf("want 2 candidates (dedup + junk filter), got %d: %+v", len(got), got)
	}
	if got[0].Host != "warsaw-homes.pl" || got[0].URL != "https://warsaw-homes.pl/" {
		t.Errorf("uddg unwrap failed: %+v", got[0])
	}
	if got[1].Host != "plainlink.example.com" {
		t.Errorf("plain link failed: %+v", got[1])
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"www.maxon.pl", "maxon.pl"},
		{"en.maxon.pl", "maxon.pl"},
		{"maxon.pl", "maxon.pl"},
		{"WWW.Foo.COM", "foo.com"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := normalizeHost(tt.host); got != tt.want {
				t.Errorf("normalizeHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestParseLeadJSON(t *testing.T) {
	decorated := "Sure! Here is the JSON:\n```json\n{\"name\":\"Acme\",\"score\":150,\"reasoning\":\"old site\"}\n```"
	l, err := parseLeadJSON(decorated)
	if err != nil {
		t.Fatalf("decorated JSON: %v", err)
	}
	if l.Name != "Acme" || l.Score != 100 {
		t.Errorf("got %+v, want Acme with score clamped to 100", l)
	}

	if l, err = parseLeadJSON(`{"skip":true}`); err != nil || !l.Skip {
		t.Errorf("skip:true should parse without name/reasoning: %v %+v", err, l)
	}

	if _, err = parseLeadJSON(`{"name":"NoReason","score":50}`); err == nil {
		t.Error("missing reasoning must be an error")
	}

	if _, err = parseLeadJSON("no json here"); err == nil {
		t.Error("garbage must be an error")
	}
}

func TestTier(t *testing.T) {
	for score, want := range map[int]string{0: "LOW", 39: "LOW", 40: "MEDIUM", 69: "MEDIUM", 70: "HIGH", 89: "HIGH", 90: "VIP", 100: "VIP"} {
		if got := tier(score); got != want {
			t.Errorf("tier(%d) = %s, want %s", score, got, want)
		}
	}
}
