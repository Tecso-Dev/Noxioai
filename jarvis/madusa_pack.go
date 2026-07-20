package main

// MADUSA packager — turns an approved MAP item (madusa.go) into a full
// FA+EN per-platform content package plus an LTX-2.3 storyboard, in exactly
// one Brain call. Rendering (madusa_render.go) consumes the storyboard.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const madusaPackSystemPrompt = `You are MADUSA's packager for NOXIOAI, turning one approved short-form video idea into a shootable package.

NOXIOAI builds practical AI employees that answer customers and automate routine work for businesses (Persian-first ICP: Iranian business owners; also EU services). NEVER invent customers, testimonials, prices, results, percentages, or statistics. Do not make guarantees.

Reply with ONLY valid JSON, no prose, in this exact shape:
{"storyboard":[{"seconds":int,"ltx_prompt":"","overlay_fa":"","overlay_en":"","vo_fa":""}],"caption_fa":"","caption_en":"","hashtags":"","titles":{"instagram":{"caption":"","hashtags":""},"tiktok":{"caption":"","hashtags":""},"youtube":{"title":"","description":"","tags":""},"linkedin":{"caption":""},"telegram":{"caption":""}}}

Storyboard rules:
- 3 to 6 scenes, 8 to 24 seconds total.
- The hook scene is first; its first 2 seconds must visually stop the scroll.
- Arc: hook → value → CTA.
- "ltx_prompt" is cinematic English in LTX-2.3 text-to-video prompt style: subject, action, camera, lighting, style.
- Overlays are short. "overlay_fa" is the primary burned-in text; "overlay_en" is secondary.
- "vo_fa" is a natural spoken Persian voiceover line for that scene — LTX-2.3 generates synced audio from it.

Caption rules: "caption_fa" is the primary caption (Persian), "caption_en" is an English variant, both platform-native length. "hashtags" mixes FA/EN tags. Restate the honest-claims rules above in your own captions — no invented numbers, customers, or guarantees.`

type madusaScene struct {
	Seconds   int    `json:"seconds"`
	LTXPrompt string `json:"ltx_prompt"`
	OverlayFA string `json:"overlay_fa"`
	OverlayEN string `json:"overlay_en"`
	VOFA      string `json:"vo_fa"`
}

type madusaIGTitles struct {
	Caption  string `json:"caption"`
	Hashtags string `json:"hashtags"`
}

type madusaYTTitles struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type madusaCaptionOnly struct {
	Caption string `json:"caption"`
}

type madusaTitles struct {
	Instagram madusaIGTitles    `json:"instagram"`
	TikTok    madusaIGTitles    `json:"tiktok"`
	YouTube   madusaYTTitles    `json:"youtube"`
	LinkedIn  madusaCaptionOnly `json:"linkedin"`
	Telegram  madusaCaptionOnly `json:"telegram"`
}

type madusaPackage struct {
	Storyboard []madusaScene `json:"storyboard"`
	CaptionFA  string        `json:"caption_fa"`
	CaptionEN  string        `json:"caption_en"`
	Hashtags   string        `json:"hashtags"`
	Titles     madusaTitles  `json:"titles"`
}

// parseMadusaPackage is pure so model-response handling can be tested
// without credentials, network access, or a database. Tolerant of prose
// wrapped around the JSON (same convention as parseMadusaAnalysis).
func parseMadusaPackage(out string) (madusaPackage, error) {
	start := strings.IndexByte(out, '{')
	if start < 0 {
		return madusaPackage{}, fmt.Errorf("no JSON found in MADUSA package")
	}
	end := strings.LastIndexByte(out, '}')
	if end <= start {
		return madusaPackage{}, fmt.Errorf("no closing brace found in MADUSA package")
	}
	var pkg madusaPackage
	if err := json.Unmarshal([]byte(out[start:end+1]), &pkg); err != nil {
		return madusaPackage{}, fmt.Errorf("decode MADUSA package JSON: %w", err)
	}
	if len(pkg.Storyboard) == 0 {
		return madusaPackage{}, fmt.Errorf("MADUSA package has no storyboard scenes")
	}
	return pkg, nil
}

// madusaStoryboardValid enforces the shootability rules a packaged
// storyboard must satisfy: 3-6 scenes, 8-24s total, and a nonempty
// ltx_prompt + vo_fa on every scene.
func madusaStoryboardValid(pkg madusaPackage) error {
	n := len(pkg.Storyboard)
	if n < 3 || n > 6 {
		return fmt.Errorf("storyboard has %d scenes, want 3-6", n)
	}
	total := 0
	for i, s := range pkg.Storyboard {
		total += s.Seconds
		if strings.TrimSpace(s.LTXPrompt) == "" {
			return fmt.Errorf("scene %d: empty ltx_prompt", i+1)
		}
		if strings.TrimSpace(s.VOFA) == "" {
			return fmt.Errorf("scene %d: empty vo_fa", i+1)
		}
	}
	if total < 8 || total > 24 {
		return fmt.Errorf("storyboard totals %ds, want 8-24s", total)
	}
	return nil
}

// formatMadusaDelivery renders the packaged content as copy-paste blocks per
// platform, for the "packed and ready" Telegram notification and (after
// rendering) the delivery report. Pure so it can be tested without a
// database or Telegram credentials.
func formatMadusaDelivery(post madusaPostRow, pkg madusaPackage) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🐍 MADUSA — Post #%d ready\n%s\n\n", post.ID, post.Idea)

	b.WriteString("— Instagram —\n")
	fmt.Fprintf(&b, "%s\n%s\n\n", pkg.Titles.Instagram.Caption, pkg.Titles.Instagram.Hashtags)

	b.WriteString("— TikTok —\n")
	fmt.Fprintf(&b, "%s\n%s\n\n", pkg.Titles.TikTok.Caption, pkg.Titles.TikTok.Hashtags)

	b.WriteString("— YouTube —\n")
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n", pkg.Titles.YouTube.Title, pkg.Titles.YouTube.Description, pkg.Titles.YouTube.Tags)

	b.WriteString("— LinkedIn —\n")
	fmt.Fprintf(&b, "%s\n\n", pkg.Titles.LinkedIn.Caption)

	b.WriteString("— Telegram —\n")
	fmt.Fprintf(&b, "%s\n\n", pkg.Titles.Telegram.Caption)

	b.WriteString("— FA/EN base captions —\n")
	fmt.Fprintf(&b, "FA: %s\nEN: %s\nHashtags: %s", pkg.CaptionFA, pkg.CaptionEN, pkg.Hashtags)
	return b.String()
}

// MadusaPack loads an approved post, makes exactly one Brain call to produce
// its full package (storyboard + per-platform captions), validates the
// storyboard, and persists it with status 'packed'.
func MadusaPack(ctx context.Context, db *sql.DB, brain *Brain, id int64) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	if brain == nil {
		return fmt.Errorf("MADUSA: brain is nil")
	}

	var idea, hook, format, status string
	err := db.QueryRowContext(ctx, `
		SELECT idea, COALESCE(hook,''), format, status FROM madusa_posts WHERE id = $1`, id,
	).Scan(&idea, &hook, &format, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("MADUSA: post #%d not found", id)
	}
	if err != nil {
		return fmt.Errorf("MADUSA: load post #%d: %w", id, err)
	}
	if status != "approved" {
		return fmt.Errorf("MADUSA: post #%d is %s, not approved", id, status)
	}

	prompt := fmt.Sprintf("Idea: %s\nHook: %s\nFormat: %s\n\nProduce the full package for this idea now.", idea, hook, format)
	messages := []Message{
		{Role: "system", Content: madusaPackSystemPrompt},
		{Role: "user", Content: prompt},
	}
	out, err := brain.Chat(messages, nil)
	if err != nil {
		return fmt.Errorf("MADUSA: pack brain call for post #%d: %w", id, err)
	}
	pkg, err := parseMadusaPackage(out)
	if err != nil {
		return fmt.Errorf("MADUSA: parse package for post #%d: %w", id, err)
	}
	if err := madusaStoryboardValid(pkg); err != nil {
		return fmt.Errorf("MADUSA: invalid storyboard for post #%d: %w", id, err)
	}

	storyboardJSON, err := json.Marshal(pkg.Storyboard)
	if err != nil {
		return fmt.Errorf("MADUSA: marshal storyboard for post #%d: %w", id, err)
	}
	titlesJSON, err := json.Marshal(pkg.Titles)
	if err != nil {
		return fmt.Errorf("MADUSA: marshal titles for post #%d: %w", id, err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE madusa_posts
		SET storyboard = $1, caption_fa = $2, caption_en = $3, hashtags = $4, titles = $5, status = 'packed'
		WHERE id = $6`,
		storyboardJSON, pkg.CaptionFA, pkg.CaptionEN, pkg.Hashtags, titlesJSON, id); err != nil {
		return fmt.Errorf("MADUSA: persist package for post #%d: %w", id, err)
	}
	return nil
}

// madusaLoadPackedPost reloads a packed post's stored package, for the
// render worker to hand to render.sh and to the delivery report.
func madusaLoadPackedPost(ctx context.Context, db *sql.DB, id int64) (madusaPostRow, madusaPackage, error) {
	var row madusaPostRow
	var pkg madusaPackage
	var storyboardJSON, titlesJSON []byte
	row.ID = id
	err := db.QueryRowContext(ctx, `
		SELECT idea, COALESCE(hook,''), format, storyboard, COALESCE(caption_fa,''), COALESCE(caption_en,''), COALESCE(hashtags,''), titles
		FROM madusa_posts WHERE id = $1`, id,
	).Scan(&row.Idea, &row.Hook, &row.Format, &storyboardJSON, &pkg.CaptionFA, &pkg.CaptionEN, &pkg.Hashtags, &titlesJSON)
	if err != nil {
		return row, pkg, fmt.Errorf("MADUSA: load packed post #%d: %w", id, err)
	}
	if len(storyboardJSON) > 0 {
		if err := json.Unmarshal(storyboardJSON, &pkg.Storyboard); err != nil {
			return row, pkg, fmt.Errorf("MADUSA: decode stored storyboard for post #%d: %w", id, err)
		}
	}
	if len(titlesJSON) > 0 {
		if err := json.Unmarshal(titlesJSON, &pkg.Titles); err != nil {
			return row, pkg, fmt.Errorf("MADUSA: decode stored titles for post #%d: %w", id, err)
		}
	}
	return row, pkg, nil
}
