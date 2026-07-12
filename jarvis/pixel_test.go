package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestPixelPromptRequestsAConciseDesignCritique(t *testing.T) {
	prompt := fmt.Sprintf(pixelPrompt, "Acme", "https://acme.test", "SaaS", "dense homepage", "weak hierarchy")
	for _, want := range []string{
		"DESIGN DIAGNOSIS",
		"3 FIXES",
		"ONE MOTION IDEA",
		"Max 160 words.",
		"dense homepage",
		"weak hierarchy",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("PIXEL prompt is missing %q", want)
		}
	}
}
