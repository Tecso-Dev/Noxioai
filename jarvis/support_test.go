package main

import "testing"

func TestShouldEscalate(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "human", text: "Can I speak to a human?", want: true},
		{name: "agent", text: "I need an agent", want: true},
		{name: "person", text: "Please connect me with a person", want: true},
		{name: "talk to someone", text: "I want to talk to someone", want: true},
		{name: "operator", text: "operator please", want: true},
		{name: "real person", text: "Is there a real person I can chat with?", want: true},
		{name: "persian representative", text: "لطفاً من را به نماینده وصل کنید", want: true},
		{name: "persian human", text: "می خواهم یک انسان پاسخ بدهد", want: true},
		{name: "pricing", text: "How much does Business Autopilot cost?", want: false},
		{name: "services", text: "Do you build websites and apps?", want: false},
		{name: "persian pricing", text: "قیمت اتوماسیون چقدر است؟", want: false},
		{name: "empty", text: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldEscalate(tt.text); got != tt.want {
				t.Errorf("shouldEscalate(%q) = %v; want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestBotGateDecision(t *testing.T) {
	cases := []struct {
		name       string
		authorized bool
		match      bool
		attempts   int
		wantAllow  bool
		wantReply  string
	}{
		{"authorized user passes silently", true, false, 0, true, ""},
		{"authorized user typing the password still passes", true, true, 0, true, ""},
		{"correct password grants access", false, true, 3, false, supportGrantedReply},
		{"wrong password gets locked prompt", false, false, 0, false, supportLockedReply},
		{"over attempt budget goes silent", false, false, supportMaxPassAttempts, false, ""},
		{"correct password after budget still grants", false, true, supportMaxPassAttempts, false, supportGrantedReply},
	}
	for _, c := range cases {
		allow, reply := botGateDecision(c.authorized, c.match, c.attempts)
		if allow != c.wantAllow || reply != c.wantReply {
			t.Errorf("%s: got (%v, %q), want (%v, %q)", c.name, allow, reply, c.wantAllow, c.wantReply)
		}
	}
}
