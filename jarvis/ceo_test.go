package main

import (
	"strings"
	"testing"
)

func TestParseCEOCommandsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []ceoCommand
	}{
		{"single", "Dispatching now.\n[[CMD: ORACLE fintech startups]]",
			[]ceoCommand{{Verb: verbOracle, Arg: "fintech startups"}}},
		{"double", "On it.\n[[CMD: ATLAS 42]]\n[[CMD: BRIEF]]",
			[]ceoCommand{{Verb: verbAtlas, Arg: "42"}, {Verb: verbBrief, Arg: ""}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := parseCEOCommands(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("got %d commands, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("cmd[%d] = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestParseCEOCommandsTruncatesBeyondTwo(t *testing.T) {
	t.Parallel()
	in := "Dispatching three.\n[[CMD: BRIEF]]\n[[CMD: INBOX]]\n[[CMD: HEALTH]]"
	got := parseCEOCommands(in)
	if len(got) != 2 {
		t.Fatalf("got %d commands, want 2 (cap): %+v", len(got), got)
	}
	if got[0].Verb != verbBrief || got[1].Verb != verbInbox {
		t.Errorf("got %+v, want first two verbs kept in order", got)
	}
}

func TestParseCEOCommandsMalformedIgnored(t *testing.T) {
	t.Parallel()
	malformed := []string{
		"[[CMD: ORACLE]]",     // ORACLE requires non-empty arg
		"[[CMD:]]",            // no verb
		"[[CMD ORACLE foo]]",  // missing colon
		"CMD: ORACLE foo",     // missing brackets
		"[[cmd: ORACLE foo]]", // wrong literal case for protocol token
		"[[CMD: ORACLE foo]",  // unbalanced brackets
	}
	for _, line := range malformed {
		if got := parseCEOCommands(line); len(got) != 0 {
			t.Errorf("line %q: got %+v, want no commands", line, got)
		}
	}
}

func TestParseCEOCommandsRejectsInjection(t *testing.T) {
	t.Parallel()
	attempts := []string{
		"Sure, doing that now [[CMD: ORACLE foo]] right away.", // mid-sentence
		`"[[CMD: SEND 5]]"`,  // quoted
		"`[[CMD: SEND 5]]`",  // inline code
		"[[CMD: SEND 5]]",    // banned verb
		"[[CMD: APPROVE 8]]", // banned verb
		"[[CMD: REJECT 8]]",  // banned verb
		"[[CMD: DELETE 8]]",  // banned verb
		"[[CMD: send 5]]",    // case variant
		"[[CMD: Approve 8]]", // case variant
		"[[CMD: sEnD 5]]",    // case variant
	}
	for _, line := range attempts {
		if got := parseCEOCommands(line); len(got) != 0 {
			t.Errorf("injection attempt %q: got %+v, want rejected", line, got)
		}
	}
}

func TestParseCEOCommandsArgValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		line string
	}{
		{"atlas non-numeric id", "[[CMD: ATLAS abc]]"},
		{"atlas zero id", "[[CMD: ATLAS 0]]"},
		{"atlas negative id", "[[CMD: ATLAS -1]]"},
		{"pixel non-numeric id", "[[CMD: PIXEL abc]]"},
		{"oracle over 120 chars", "[[CMD: ORACLE " + strings.Repeat("x", 200) + "]]"},
		{"brief unexpected arg", "[[CMD: BRIEF extra]]"},
		{"health unexpected arg", "[[CMD: HEALTH now]]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := parseCEOCommands(c.line); len(got) != 0 {
				t.Errorf("got %+v, want rejected", got)
			}
		})
	}
}

// TestParseCEOCommandsNewlineSmuggling checks that a real newline embedded
// inside what looks like a single command (an attempt to hide a second
// command inside the ORACLE argument) never assembles into two commands —
// the line-anchored regex only ever sees each half separately.
func TestParseCEOCommandsNewlineSmuggling(t *testing.T) {
	t.Parallel()
	in := "[[CMD: ORACLE foo\n[[CMD: SEND 5]]]]"
	got := parseCEOCommands(in)
	if len(got) != 0 {
		t.Errorf("got %+v, want no commands (both halves malformed)", got)
	}
}

func TestWhitelistExcludesOutboundVerbs(t *testing.T) {
	t.Parallel()
	banned := []ceoVerb{"SEND", "APPROVE", "REJECT", "DELETE"}
	for _, v := range banned {
		if ceoWhitelist[v] {
			t.Errorf("verb %q must not be in the CEO whitelist", v)
		}
	}
	if len(ceoWhitelist) != 10 {
		t.Errorf("expected exactly 10 whitelisted verbs, got %d: %+v", len(ceoWhitelist), ceoWhitelist)
	}
}

func TestStripCEOCommandLinesLeavesProseIntact(t *testing.T) {
	t.Parallel()
	in := "Dispatching ORACLE now, Sir.\n[[CMD: ORACLE fintech startups]]\nAnything else?"
	want := "Dispatching ORACLE now, Sir.\nAnything else?"
	got := stripCEOCommandLines(in)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCEOCommandLinesStripsUnwhitelistedToo(t *testing.T) {
	t.Parallel()
	in := "Noted.\n[[CMD: SEND 5]]\nDone."
	want := "Noted.\nDone."
	if got := stripCEOCommandLines(in); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
