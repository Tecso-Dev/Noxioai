package main

import (
	"fmt"
	"testing"
)

func TestParseFollowupJSON(t *testing.T) {
	body := "I wanted to briefly follow up on my note about improving your customer journey. We recently helped a similar team reduce the manual steps between first visit and qualified enquiry, which made their existing traffic work harder. It can be a useful way to learn where prospects are dropping before they ever reach the sales conversation. If that is on your roadmap, would a short conversation next week be useful?\n\nSobhan — NOXIOAI"

	draft, err := parseFollowupJSON(fmt.Sprintf("Here is the draft:\n```json\n{\"subject\":%q,\"body\":%q}\n```", "A quick follow-up on your customer journey", body))
	if err != nil {
		t.Fatalf("valid follow-up rejected: %v", err)
	}
	if draft.Subject != "A quick follow-up on your customer journey" || draft.Body != body {
		t.Fatalf("parsed draft = %#v, want supplied subject and body", draft)
	}
}
