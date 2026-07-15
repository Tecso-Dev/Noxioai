package main

import (
	"strings"
	"testing"
)

func TestStripThought(t *testing.T) {
	cases := [][2]string{
		{"<thought>hidden</thought>visible", "visible"},
		{"  \n<thought>multi\nline</thought>\n answer", "answer"},
		{"no block at all", "no block at all"},
		{"<thought>unterminated", "<thought>unterminated"},
	}
	for _, c := range cases {
		if got := stripThought(c[0]); got != c[1] {
			t.Errorf("stripThought(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestThoughtFilterSplitTokens(t *testing.T) {
	var out strings.Builder
	f := &thoughtFilter{emit: func(s string) { out.WriteString(s) }}
	// tags split across token boundaries, as a stream delivers them
	for _, tok := range []string{"<tho", "ught>secret ", "stuff</tho", "ught>Hello", " world"} {
		f.feed(tok)
	}
	if out.String() != "Hello world" {
		t.Errorf("filtered stream = %q, want %q", out.String(), "Hello world")
	}

	out.Reset()
	f = &thoughtFilter{emit: func(s string) { out.WriteString(s) }}
	for _, tok := range []string{"<though", "tful reply, no block"} {
		f.feed(tok)
	}
	if out.String() != "<thoughtful reply, no block" {
		t.Errorf("passthrough stream = %q, want %q", out.String(), "<thoughtful reply, no block")
	}
}
