package main

import (
	"strings"
	"testing"
)

func TestFormatSocialCaption(t *testing.T) {
	tests := []struct {
		name     string
		caption  string
		hashtags []string
		want     string
	}{
		{
			name:     "normalizes Persian hashtags",
			caption:  "  پاسخ‌گویی مشتری‌ها را به همکار هوشمند بسپارید.  ",
			hashtags: []string{"هوش مصنوعی", "#کسب_و_کار"},
			want:     "پاسخ‌گویی مشتری‌ها را به همکار هوشمند بسپارید.\n\n#هوش_مصنوعی #کسب_و_کار",
		},
		{
			name:     "deduplicates and caps hashtags",
			caption:  "کسب‌وکارتان حتی شب‌ها پاسخ‌گو می‌ماند.",
			hashtags: []string{"#ناکسیو", "ناکسیو", "هوش_مصنوعی", "اتوماسیون"},
			want:     "کسب‌وکارتان حتی شب‌ها پاسخ‌گو می‌ماند.\n\n#ناکسیو #هوش_مصنوعی #اتوماسیون",
		},
		{
			name:     "adds safe defaults when model omits hashtags",
			caption:  "یک پاسخ سریع، شروع یک تجربه بهتر است.",
			hashtags: nil,
			want:     "یک پاسخ سریع، شروع یک تجربه بهتر است.\n\n#هوش_مصنوعی #ناکسیو",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatSocialCaption(tt.caption, tt.hashtags); got != tt.want {
				t.Fatalf("formatSocialCaption() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestParseSocialDrafts(t *testing.T) {
	input := "```json\n" + `{
  "posts": [
    {
      "caption": "مشتری نیمه‌شب هم سؤال دارد؛ همکار هوشمند پاسخ‌گو می‌ماند.",
      "hashtags": ["هوش_مصنوعی", "پشتیبانی_مشتری"],
      "image_prompt": "A warm, modern customer support scene at night"
    },
    {
      "caption": "برای دایرکت فروشگاه، پاسخ‌های روشن و کوتاه آماده کنید.",
      "hashtags": ["فروشگاه_آنلاین", "اینستاگرام", "اتوماسیون"],
      "image_prompt": "An Iranian online shop owner organizing messages"
    },
    {
      "caption": "وقتی شما استراحت می‌کنید، مسیر پاسخ‌گویی متوقف نمی‌شود.",
      "hashtags": ["کسب_و_کار", "ناکسیو"],
      "image_prompt": "A calm sleeping city with an always-on AI assistant"
    }
  ]
}` + "\n```"

	drafts, err := parseSocialDrafts(input)
	if err != nil {
		t.Fatalf("parseSocialDrafts() error = %v", err)
	}
	if len(drafts) != socialDraftCount {
		t.Fatalf("parseSocialDrafts() returned %d drafts; want %d", len(drafts), socialDraftCount)
	}
	if drafts[0].Platform != "telegram" || drafts[1].Platform != "instagram" || drafts[2].Platform != "telegram" {
		t.Fatalf("unexpected platform sequence: %q, %q, %q", drafts[0].Platform, drafts[1].Platform, drafts[2].Platform)
	}
	if !strings.Contains(drafts[0].Caption, "#هوش_مصنوعی #پشتیبانی_مشتری") {
		t.Fatalf("first caption missing formatted hashtags: %q", drafts[0].Caption)
	}
	if drafts[2].ImagePrompt != "A calm sleeping city with an always-on AI assistant" {
		t.Fatalf("third image prompt = %q", drafts[2].ImagePrompt)
	}
}

func TestParseSocialDraftsRejectsInvalidBatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "not JSON", input: "سه پست آماده شد"},
		{name: "wrong count", input: `{"posts":[{"caption":"متن","hashtags":["یک","دو"],"image_prompt":"prompt"}]}`},
		{name: "empty caption", input: `{"posts":[{"caption":"","image_prompt":"one"},{"caption":"two","image_prompt":"two"},{"caption":"three","image_prompt":"three"}]}`},
		{name: "empty prompt", input: `{"posts":[{"caption":"one","image_prompt":"one"},{"caption":"two","image_prompt":""},{"caption":"three","image_prompt":"three"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseSocialDrafts(tt.input); err == nil {
				t.Fatal("parseSocialDrafts() error = nil; want error")
			}
		})
	}
}
