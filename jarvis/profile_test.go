package main

import "testing"

func TestNormalizeBusinessProfile(t *testing.T) {
	input := businessProfile{
		BusinessName:  "  Noxio Bakery  ",
		Sells:         "  artisan bread  ",
		IdealCustomer: "  local families  ",
		City:          "  Tehran  ",
		Country:       "  Iran  ",
		Language:      "  fa  ",
		Website:       "  https://example.com  ",
		Telegram:      "  @noxio  ",
		Knowledge:     "  Q: Delivery? A: Same day.  ",
		Goals:         "  answer customers faster  ",
	}

	got, err := normalizeBusinessProfile(input)
	if err != nil {
		t.Fatalf("normalizeBusinessProfile() error = %v", err)
	}
	if got.BusinessName != "Noxio Bakery" || got.Sells != "artisan bread" ||
		got.IdealCustomer != "local families" || got.City != "Tehran" ||
		got.Country != "Iran" || got.Language != "fa" ||
		got.Website != "https://example.com" || got.Telegram != "@noxio" ||
		got.Knowledge != "Q: Delivery? A: Same day." ||
		got.Goals != "answer customers faster" {
		t.Fatalf("normalizeBusinessProfile() = %#v; fields were not trimmed", got)
	}
}

func TestNormalizeBusinessProfileRequiresCoreKnowledge(t *testing.T) {
	tests := []struct {
		name    string
		profile businessProfile
	}{
		{name: "business name", profile: businessProfile{Knowledge: "Returns are accepted within 7 days."}},
		{name: "knowledge", profile: businessProfile{BusinessName: "Noxio Bakery"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeBusinessProfile(tt.profile); err == nil {
				t.Fatal("normalizeBusinessProfile() error = nil, want validation error")
			}
		})
	}
}
