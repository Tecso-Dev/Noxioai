package main

import (
	"math"
	"strings"
	"testing"
)

func TestMadusaCreatorMomentum(t *testing.T) {
	const epsilon = 1e-9
	tests := []struct {
		name           string
		subsPrev       int64
		subsCur        int64
		viewsPrev      int64
		viewsCur       int64
		days           float64
		postsRecent14d int
		want           float64
	}{
		{
			name:     "zero growth, baseline posting cadence (6/14d = 3/wk)",
			subsPrev: 1000, subsCur: 1000, viewsPrev: 1000, viewsCur: 1000,
			days: 1, postsRecent14d: 6,
			want: 0.2, // subGrowth 0 + viewGrowth 0 + postScore(1) * 0.2
		},
		{
			name:     "10% sub and view growth over 1 day, baseline posting",
			subsPrev: 1000, subsCur: 1100, viewsPrev: 10000, viewsCur: 11000,
			days: 1, postsRecent14d: 6,
			want: 0.28, // 0.1*0.4 + 0.1*0.4 + 1*0.2
		},
		{
			name:     "zero prev subs/views guards div-by-zero",
			subsPrev: 0, subsCur: 500, viewsPrev: 0, viewsCur: 5000,
			days: 1, postsRecent14d: 0,
			want: 0,
		},
		{
			name:     "negative days guarded to 1 day",
			subsPrev: 1000, subsCur: 1100, viewsPrev: 1000, viewsCur: 1100,
			days: -5, postsRecent14d: 0,
			want: 0.08, // 0.1*0.4 + 0.1*0.4 + 0
		},
		{
			name:     "negative posting count guarded to zero posting score",
			subsPrev: 1000, subsCur: 1000, viewsPrev: 1000, viewsCur: 1000,
			days: 1, postsRecent14d: -5,
			want: 0,
		},
		{
			name:     "runaway posting cadence caps posting score at 2x",
			subsPrev: 1000, subsCur: 1000, viewsPrev: 1000, viewsCur: 1000,
			days: 1, postsRecent14d: 28, // 28/14d*7 = 14/wk -> 14/3 uncapped, capped to 2
			want: 0.4, // 2 * 0.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := madusaCreatorMomentum(tt.subsPrev, tt.subsCur, tt.viewsPrev, tt.viewsCur, tt.days, tt.postsRecent14d)
			if math.Abs(got-tt.want) > epsilon {
				t.Fatalf("madusaCreatorMomentum() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestMadusaTrendStage(t *testing.T) {
	tests := []struct {
		name     string
		velocity float64
		accel    float64
		want     string
	}{
		{"negative velocity always dying, even with rising accel", -1, 5, "dying"},
		{"zero velocity boundary is dying", 0, 1, "dying"},
		{"positive velocity, decelerating is peaking", 1, -1, "peaking"},
		{"velocity at the small/large boundary (0.5) is growing", 0.5, 0, "growing"},
		{"large velocity, flat accel is growing", 1, 0, "growing"},
		{"small velocity but accelerating is growing", 0.3, 1, "growing"},
		{"small velocity, flat accel is emerging", 0.3, 0, "emerging"},
		{"just under the small/large boundary but accelerating is growing", 0.49, 0.01, "growing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := madusaTrendStage(tt.velocity, tt.accel); got != tt.want {
				t.Fatalf("madusaTrendStage(%v, %v) = %q; want %q", tt.velocity, tt.accel, got, tt.want)
			}
		})
	}
}

func TestMadusaSignalVelocity(t *testing.T) {
	tests := []struct {
		name         string
		scorePrev    int
		scoreCur     int
		hoursBetween float64
		want         float64
	}{
		{"rising score over 2 hours", 10, 20, 2, 5},
		{"falling score over 2 hours", 20, 10, 2, -5},
		{"zero hours window guarded to 1 hour", 0, 5, 0, 5},
		{"negative hours window guarded to 1 hour", 0, 3, -2, 3},
		{"unchanged score is zero velocity", 10, 10, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := madusaSignalVelocity(tt.scorePrev, tt.scoreCur, tt.hoursBetween); got != tt.want {
				t.Fatalf("madusaSignalVelocity() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestParseMadusaAnalysis(t *testing.T) {
	validJSON := `{"trends":[{"topic":"AI agents building SaaS","stage":"growing","why":"multiple creators posting demos this week","hook_patterns":"curiosity gap"}],"map":[{"idea":"Show a business owner texting an AI at midnight and getting instant help","hook":"(question) What if your business never slept?","format":"reel","platforms":["reel","short"],"why_now":"AI-agent building content is spiking across creators","tie_to_noxioai":"NOXIOAI builds exactly this kind of always-on AI employee for businesses"}]}`

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		analysis, err := parseMadusaAnalysis(validJSON)
		if err != nil {
			t.Fatalf("parseMadusaAnalysis() error = %v", err)
		}
		if len(analysis.Trends) != 1 || analysis.Trends[0].Topic != "AI agents building SaaS" {
			t.Fatalf("unexpected trends: %+v", analysis.Trends)
		}
		if len(analysis.Map) != 1 || analysis.Map[0].Format != "reel" {
			t.Fatalf("unexpected map: %+v", analysis.Map)
		}
	})

	t.Run("wrapped in prose and a fenced code block", func(t *testing.T) {
		t.Parallel()
		wrapped := "Here is the analysis:\n```json\n" + validJSON + "\n```\nLet me know if you need anything else."
		analysis, err := parseMadusaAnalysis(wrapped)
		if err != nil {
			t.Fatalf("parseMadusaAnalysis() error = %v", err)
		}
		if len(analysis.Map) != 1 || analysis.Map[0].Idea == "" {
			t.Fatalf("unexpected map after unwrapping prose: %+v", analysis.Map)
		}
	})

	t.Run("garbage rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := parseMadusaAnalysis("I cannot comply with this request."); err == nil {
			t.Fatal("parseMadusaAnalysis() error = nil; want error for non-JSON garbage")
		}
	})

	t.Run("empty map rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := parseMadusaAnalysis(`{"trends":[],"map":[]}`); err == nil {
			t.Fatal("parseMadusaAnalysis() error = nil; want error for empty map")
		}
	})
}

func TestFormatMadusaMapReport(t *testing.T) {
	rows := []madusaPostRow{
		{ID: 7, Idea: "Idea one", Hook: "(number) 3 things nobody tells you", Format: "reel", Stage: "growing", WhyNow: "spiking on YouTube"},
		{ID: 8, Idea: "Idea two", Hook: "(question) What if?", Format: "short", Stage: "emerging", WhyNow: "early signal on Reddit"},
	}
	report := formatMadusaMapReport(rows)

	if !strings.Contains(report, "🐍") {
		t.Fatalf("report missing snake emoji header: %q", report)
	}
	for _, want := range []string{"#7", "#8", "Idea one", "Idea two", "jarvis madusa approve <id>", "jarvis madusa reject <id>"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q: %q", want, report)
		}
	}
}

func TestFormatMadusaMapReportEmpty(t *testing.T) {
	report := formatMadusaMapReport(nil)
	if !strings.Contains(report, "No proposed items") {
		t.Fatalf("empty report should say so: %q", report)
	}
}
