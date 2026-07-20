package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ── madusa_pack.go ───────────────────────────────────────────────────────

func TestParseMadusaPackage(t *testing.T) {
	valid := `{"storyboard":[{"seconds":3,"ltx_prompt":"a","overlay_fa":"x","overlay_en":"y","vo_fa":"v"}],"caption_fa":"fa","caption_en":"en","hashtags":"#a","titles":{"instagram":{"caption":"ig","hashtags":"#ig"},"tiktok":{"caption":"tt","hashtags":"#tt"},"youtube":{"title":"yt","description":"d","tags":"t"},"linkedin":{"caption":"li"},"telegram":{"caption":"tg"}}}`

	t.Run("valid", func(t *testing.T) {
		pkg, err := parseMadusaPackage(valid)
		if err != nil {
			t.Fatalf("parseMadusaPackage() error = %v", err)
		}
		if len(pkg.Storyboard) != 1 || pkg.Storyboard[0].LTXPrompt != "a" {
			t.Fatalf("parseMadusaPackage() = %+v", pkg)
		}
		if pkg.Titles.Instagram.Caption != "ig" || pkg.Titles.YouTube.Title != "yt" {
			t.Fatalf("parseMadusaPackage() titles = %+v", pkg.Titles)
		}
	})

	t.Run("wrapped in prose", func(t *testing.T) {
		wrapped := "Sure, here you go:\n```json\n" + valid + "\n```\nHope that helps!"
		pkg, err := parseMadusaPackage(wrapped)
		if err != nil {
			t.Fatalf("parseMadusaPackage() error = %v", err)
		}
		if len(pkg.Storyboard) != 1 {
			t.Fatalf("parseMadusaPackage() = %+v", pkg)
		}
	})

	t.Run("garbage", func(t *testing.T) {
		if _, err := parseMadusaPackage("not json at all"); err == nil {
			t.Fatal("parseMadusaPackage() error = nil, want error")
		}
	})

	t.Run("missing storyboard", func(t *testing.T) {
		if _, err := parseMadusaPackage(`{"caption_fa":"fa","caption_en":"en"}`); err == nil {
			t.Fatal("parseMadusaPackage() error = nil, want error")
		}
	})

	t.Run("unclosed brace is garbage", func(t *testing.T) {
		if _, err := parseMadusaPackage(`{"storyboard":[`); err == nil {
			t.Fatal("parseMadusaPackage() error = nil, want error")
		}
	})
}

func TestMadusaStoryboardValid(t *testing.T) {
	scene := func(seconds int, ltx, vo string) madusaScene {
		return madusaScene{Seconds: seconds, LTXPrompt: ltx, VOFA: vo}
	}

	tests := []struct {
		name    string
		scenes  []madusaScene
		wantErr bool
	}{
		{
			name: "valid 3 scenes totalling 12s",
			scenes: []madusaScene{
				scene(4, "hook shot", "سلام"),
				scene(4, "value shot", "این کار رو ببین"),
				scene(4, "cta shot", "همین حالا شروع کن"),
			},
			wantErr: false,
		},
		{
			name:    "too few scenes (2)",
			scenes:  []madusaScene{scene(4, "a", "v"), scene(4, "b", "v")},
			wantErr: true,
		},
		{
			name: "too many scenes (7)",
			scenes: []madusaScene{
				scene(2, "a", "v"), scene(2, "b", "v"), scene(2, "c", "v"), scene(2, "d", "v"),
				scene(2, "e", "v"), scene(2, "f", "v"), scene(2, "g", "v"),
			},
			wantErr: true,
		},
		{
			name: "total seconds too short (6s over 3 scenes)",
			scenes: []madusaScene{
				scene(2, "a", "v"), scene(2, "b", "v"), scene(2, "c", "v"),
			},
			wantErr: true,
		},
		{
			name: "total seconds too long (30s over 3 scenes)",
			scenes: []madusaScene{
				scene(10, "a", "v"), scene(10, "b", "v"), scene(10, "c", "v"),
			},
			wantErr: true,
		},
		{
			name: "empty ltx_prompt on one scene",
			scenes: []madusaScene{
				scene(4, "a", "v"), scene(4, "", "v"), scene(4, "c", "v"),
			},
			wantErr: true,
		},
		{
			name: "empty vo_fa on one scene",
			scenes: []madusaScene{
				scene(4, "a", "v"), scene(4, "b", "   "), scene(4, "c", "v"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := madusaStoryboardValid(madusaPackage{Storyboard: tt.scenes})
			if (err != nil) != tt.wantErr {
				t.Fatalf("madusaStoryboardValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatMadusaDelivery(t *testing.T) {
	post := madusaPostRow{ID: 42, Idea: "AI agents replacing support tickets"}
	pkg := madusaPackage{
		CaptionFA: "کپشن فارسی",
		CaptionEN: "english caption",
		Hashtags:  "#ai #خودکارسازی",
		Titles: madusaTitles{
			Instagram: madusaIGTitles{Caption: "ig-caption", Hashtags: "#ig"},
			TikTok:    madusaIGTitles{Caption: "tt-caption", Hashtags: "#tt"},
			YouTube:   madusaYTTitles{Title: "yt-title", Description: "yt-desc", Tags: "yt,tags"},
			LinkedIn:  madusaCaptionOnly{Caption: "li-caption"},
			Telegram:  madusaCaptionOnly{Caption: "tg-caption"},
		},
	}

	out := formatMadusaDelivery(post, pkg)

	for _, want := range []string{
		"#42", "AI agents replacing support tickets",
		"Instagram", "ig-caption", "#ig",
		"TikTok", "tt-caption", "#tt",
		"YouTube", "yt-title", "yt-desc", "yt,tags",
		"LinkedIn", "li-caption",
		"Telegram", "tg-caption",
		"کپشن فارسی", "english caption", "#ai #خودکارسازی",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("formatMadusaDelivery() missing %q in:\n%s", want, out)
		}
	}
}

// ── madusa_render.go ─────────────────────────────────────────────────────

func TestMadusaPickGPUPlan(t *testing.T) {
	t.Run("picks cheapest l40s plan", func(t *testing.T) {
		fixture := `{"plans":[
			{"id":"vc2-2c-4gb","monthly_cost":20},
			{"id":"vcg-a16-2c-16g","monthly_cost":500},
			{"id":"vcg-l40s-8c-64g","monthly_cost":1500},
			{"id":"vcg-l40s-4c-32g","monthly_cost":800}
		]}`
		plan, err := madusaPickGPUPlan([]byte(fixture))
		if err != nil {
			t.Fatalf("madusaPickGPUPlan() error = %v", err)
		}
		if plan != "vcg-l40s-4c-32g" {
			t.Fatalf("madusaPickGPUPlan() = %q, want vcg-l40s-4c-32g", plan)
		}
	})

	t.Run("no l40s plan errors", func(t *testing.T) {
		fixture := `{"plans":[{"id":"vcg-a16-2c-16g","monthly_cost":500}]}`
		if _, err := madusaPickGPUPlan([]byte(fixture)); err == nil {
			t.Fatal("madusaPickGPUPlan() error = nil, want error")
		}
	})

	t.Run("garbage json errors", func(t *testing.T) {
		if _, err := madusaPickGPUPlan([]byte("not json")); err == nil {
			t.Fatal("madusaPickGPUPlan() error = nil, want error")
		}
	})
}

func TestMadusaParseInstance(t *testing.T) {
	t.Run("valid instance", func(t *testing.T) {
		fixture := `{"instance":{"id":"abc-123","main_ip":"1.2.3.4","status":"active","label":"madusa-render-1690000000","date_created":"2026-07-20T10:00:00+00:00"}}`
		id, ip, status, err := madusaParseInstance([]byte(fixture))
		if err != nil {
			t.Fatalf("madusaParseInstance() error = %v", err)
		}
		if id != "abc-123" || ip != "1.2.3.4" || status != "active" {
			t.Fatalf("madusaParseInstance() = (%q, %q, %q)", id, ip, status)
		}
	})

	t.Run("missing id errors", func(t *testing.T) {
		if _, _, _, err := madusaParseInstance([]byte(`{"instance":{"main_ip":"1.2.3.4"}}`)); err == nil {
			t.Fatal("madusaParseInstance() error = nil, want error")
		}
	})

	t.Run("garbage json errors", func(t *testing.T) {
		if _, _, _, err := madusaParseInstance([]byte("not json")); err == nil {
			t.Fatal("madusaParseInstance() error = nil, want error")
		}
	})
}

func TestMadusaOrphanIDs(t *testing.T) {
	now := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	maxAge := 3 * time.Hour

	t.Run("label timestamp older than max age is orphaned", func(t *testing.T) {
		old := now.Add(-4 * time.Hour).Unix()
		fixture := `{"instances":[{"id":"old-1","label":"madusa-render-` + strconv.FormatInt(old, 10) + `"}]}`
		ids := madusaOrphanIDs([]byte(fixture), now, maxAge)
		if len(ids) != 1 || ids[0] != "old-1" {
			t.Fatalf("madusaOrphanIDs() = %v, want [old-1]", ids)
		}
	})

	t.Run("label timestamp within max age is not orphaned", func(t *testing.T) {
		recent := now.Add(-1 * time.Hour).Unix()
		fixture := `{"instances":[{"id":"fresh-1","label":"madusa-render-` + strconv.FormatInt(recent, 10) + `"}]}`
		ids := madusaOrphanIDs([]byte(fixture), now, maxAge)
		if len(ids) != 0 {
			t.Fatalf("madusaOrphanIDs() = %v, want []", ids)
		}
	})

	t.Run("falls back to date_created when label is unparseable", func(t *testing.T) {
		fixture := `{"instances":[{"id":"legacy-1","label":"some-other-label","date_created":"2026-07-20T10:00:00+00:00"}]}`
		ids := madusaOrphanIDs([]byte(fixture), now, maxAge) // 5h old, over 3h budget
		if len(ids) != 1 || ids[0] != "legacy-1" {
			t.Fatalf("madusaOrphanIDs() = %v, want [legacy-1]", ids)
		}
	})

	t.Run("no usable timestamp is left alone, not destroyed blind", func(t *testing.T) {
		fixture := `{"instances":[{"id":"unknown-1","label":"some-other-label"}]}`
		ids := madusaOrphanIDs([]byte(fixture), now, maxAge)
		if len(ids) != 0 {
			t.Fatalf("madusaOrphanIDs() = %v, want []", ids)
		}
	})

	t.Run("garbage json returns nil, not a crash", func(t *testing.T) {
		if ids := madusaOrphanIDs([]byte("not json"), now, maxAge); ids != nil {
			t.Fatalf("madusaOrphanIDs() = %v, want nil", ids)
		}
	})
}

func TestMadusaRenderDeadline(t *testing.T) {
	start := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	t.Run("normal hours", func(t *testing.T) {
		got := madusaRenderDeadline(start, 2)
		want := start.Add(2 * time.Hour)
		if !got.Equal(want) {
			t.Fatalf("madusaRenderDeadline() = %v, want %v", got, want)
		}
	})

	t.Run("zero hours guarded to 3h default", func(t *testing.T) {
		got := madusaRenderDeadline(start, 0)
		want := start.Add(3 * time.Hour)
		if !got.Equal(want) {
			t.Fatalf("madusaRenderDeadline() = %v, want %v", got, want)
		}
	})

	t.Run("negative hours guarded to 3h default", func(t *testing.T) {
		got := madusaRenderDeadline(start, -5)
		want := start.Add(3 * time.Hour)
		if !got.Equal(want) {
			t.Fatalf("madusaRenderDeadline() = %v, want %v", got, want)
		}
	})
}

func TestMadusaElapsedHours(t *testing.T) {
	start := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	got := madusaElapsedHours(start, end)
	if got != 1.5 {
		t.Fatalf("madusaElapsedHours() = %v, want 1.5", got)
	}
}

func TestMadusaTruncate(t *testing.T) {
	t.Parallel()
	fa := strings.Repeat("سلام دنیا ", 30) // multi-byte Persian, 300 runes
	got := madusaTruncate(fa, 200)
	if len([]rune(got)) != 200 {
		t.Errorf("want 200 runes, got %d", len([]rune(got)))
	}
	if !utf8.ValidString(got) {
		t.Error("truncated string is not valid UTF-8")
	}
	if s := madusaTruncate("short", 200); s != "short" {
		t.Errorf("short string mutated: %q", s)
	}
}
