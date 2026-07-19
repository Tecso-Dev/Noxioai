package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	socialDraftCount        = 3
	socialBrainGuardMessage = "Social agent: set JARVIS_API_KEY (plus the DeepSeek JARVIS_BASE_URL and JARVIS_MODEL) to enable"
	socialFALEndpoint       = "https://fal.run/fal-ai/flux/schnell"
)

const socialSystemPrompt = `You are NOXIOAI's Persian-first social content editor.
NOXIOAI builds practical AI employees that help businesses answer customers and automate routine work, including while the business owner is offline.

Write useful, credible marketing content for Iranian business owners. Be warm, specific, and easy to understand. Never invent customers, testimonials, prices, results, percentages, or statistics. Do not make guarantees. Do not claim that a generated image contains a real customer or real NOXIOAI product interface.`

var socialPlatforms = [socialDraftCount]string{"telegram", "instagram", "telegram"}

type socialDraft struct {
	Platform    string
	Caption     string
	ImagePrompt string
	ImageURL    string
}

type socialModelDraft struct {
	Caption     string   `json:"caption"`
	Hashtags    []string `json:"hashtags"`
	ImagePrompt string   `json:"image_prompt"`
}

type socialApproval struct {
	ID            int64
	Platform      string
	Status        string
	Published     bool
	AlreadyPosted bool
}

// RunSocial performs one content cycle: generate three Persian drafts, create
// optional images, persist every draft, then send each one to the owner for a
// human decision. The configuration guard deliberately runs before the DB is
// checked so this command is safe on hosts where the agent is not set up yet.
func RunSocial(ctx context.Context, db *sql.DB) error {
	if !socialBrainConfigured() {
		log.Print(socialBrainGuardMessage)
		return nil
	}
	if db == nil {
		return fmt.Errorf("Social agent: database is nil")
	}

	drafts, err := generateSocialDrafts(ctx, NewBrainFromEnv())
	if err != nil {
		return fmt.Errorf("Social agent: generate drafts with Brain: %w", err)
	}

	falKey := strings.TrimSpace(os.Getenv("FAL_KEY"))
	var falClient *http.Client
	if falKey != "" {
		falClient = &http.Client{Timeout: 2 * time.Minute}
	}
	ids := make([]int64, len(drafts))
	for i := range drafts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if falKey != "" {
			imageURL, imageErr := generateSocialImage(ctx, falClient, falKey, drafts[i].ImagePrompt)
			if imageErr != nil {
				log.Printf("Social agent: image generation for draft %d: %v", i+1, imageErr)
			} else {
				drafts[i].ImageURL = imageURL
			}
		}

		if err := db.QueryRowContext(ctx, `
			INSERT INTO social_posts (platform, caption, image_url, status)
			VALUES ($1, $2, $3, 'draft')
			RETURNING id`, drafts[i].Platform, drafts[i].Caption, drafts[i].ImageURL).Scan(&ids[i]); err != nil {
			return fmt.Errorf("Social agent: store %s draft: %w", drafts[i].Platform, err)
		}
	}

	for i, draft := range drafts {
		if err := SendTelegram(formatSocialReview(ids[i], draft)); err != nil {
			return fmt.Errorf("Social agent: deliver draft #%d to owner: %w", ids[i], err)
		}
	}
	log.Printf("Social agent: %d drafts stored and delivered for approval", len(drafts))
	return nil
}

// socialBrainConfigured treats the hosted model credential as the setup
// switch. NewBrainFromEnv has a local Ollama fallback for interactive use, but
// scheduled marketing must not unexpectedly probe a local service.
func socialBrainConfigured() bool {
	return strings.TrimSpace(os.Getenv("JARVIS_API_KEY")) != ""
}

func generateSocialDrafts(ctx context.Context, brain *Brain) ([]socialDraft, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prompt := `Create exactly three Persian social posts in this exact order:
1. NOXIOAI AI employees answering customers 24/7.
2. Practical advice for Iranian shops or clinics handling customers on Instagram and Telegram.
3. The honest "your business never sleeps" angle: automation keeps routine customer responses moving while the owner rests.

For every post:
- Write a natural Persian caption of roughly 250-600 characters.
- Keep claims honest; use no fabricated stats, clients, testimonials, or guaranteed outcomes.
- Supply 2-3 relevant Persian hashtags separately from the caption.
- Supply one detailed English image prompt for a clean, modern square social visual. Avoid logos, UI screenshots, and text inside the image.

Return ONLY valid JSON with this exact shape:
{"posts":[{"caption":"","hashtags":["",""],"image_prompt":""},{"caption":"","hashtags":["",""],"image_prompt":""},{"caption":"","hashtags":["",""],"image_prompt":""}]}`
	messages := []Message{
		{Role: "system", Content: socialSystemPrompt},
		{Role: "user", Content: prompt},
	}
	out, err := brain.Chat(messages, nil)
	if err != nil {
		return nil, err
	}
	drafts, parseErr := parseSocialDrafts(out)
	if parseErr == nil {
		return drafts, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err = brain.Chat(append(messages,
		Message{Role: "assistant", Content: out},
		Message{Role: "user", Content: "The response was invalid: " + parseErr.Error() + ". Reply again with ONLY valid JSON in the required shape."},
	), nil)
	if err != nil {
		return nil, err
	}
	return parseSocialDrafts(out)
}

// parseSocialDrafts is pure so model-response handling can be tested without
// credentials, network access, or a database.
func parseSocialDrafts(out string) ([]socialDraft, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object found")
	}
	var response struct {
		Posts []socialModelDraft `json:"posts"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &response); err != nil {
		return nil, fmt.Errorf("decode drafts JSON: %w", err)
	}
	if len(response.Posts) != socialDraftCount {
		return nil, fmt.Errorf("expected %d posts, got %d", socialDraftCount, len(response.Posts))
	}

	drafts := make([]socialDraft, 0, socialDraftCount)
	for i, post := range response.Posts {
		caption := strings.TrimSpace(post.Caption)
		imagePrompt := strings.TrimSpace(post.ImagePrompt)
		if caption == "" {
			return nil, fmt.Errorf("post %d caption is empty", i+1)
		}
		if imagePrompt == "" {
			return nil, fmt.Errorf("post %d image_prompt is empty", i+1)
		}
		drafts = append(drafts, socialDraft{
			Platform:    socialPlatforms[i],
			Caption:     formatSocialCaption(caption, post.Hashtags),
			ImagePrompt: imagePrompt,
		})
	}
	return drafts, nil
}

// formatSocialCaption produces a stable caption with two or three hashtags.
// Safe Persian defaults cover the occasional model response that omits them.
func formatSocialCaption(caption string, hashtags []string) string {
	caption = strings.TrimSpace(caption)
	normalized := make([]string, 0, 3)
	seen := make(map[string]struct{})
	add := func(tag string) {
		tag = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
		tag = strings.Join(strings.Fields(tag), "_")
		tag = strings.Trim(tag, "#،,؛;.")
		if tag == "" || len(normalized) == 3 {
			return
		}
		tag = "#" + tag
		if _, exists := seen[tag]; exists {
			return
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	for _, hashtag := range hashtags {
		add(hashtag)
	}
	for _, fallback := range []string{"هوش_مصنوعی", "ناکسیو"} {
		if len(normalized) >= 2 {
			break
		}
		add(fallback)
	}
	return caption + "\n\n" + strings.Join(normalized, " ")
}

func generateSocialImage(ctx context.Context, client *http.Client, falKey, prompt string) (string, error) {
	body, err := json.Marshal(struct {
		Prompt string `json:"prompt"`
	}{Prompt: prompt})
	if err != nil {
		return "", fmt.Errorf("encode fal.ai request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, socialFALEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create fal.ai request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+falKey)
	req.Header.Set("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("read fal.ai response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("fal.ai returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	var result struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Image struct {
			URL string `json:"url"`
		} `json:"image"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("decode fal.ai response: %w", err)
	}
	if len(result.Images) > 0 && strings.TrimSpace(result.Images[0].URL) != "" {
		return strings.TrimSpace(result.Images[0].URL), nil
	}
	if strings.TrimSpace(result.Image.URL) != "" {
		return strings.TrimSpace(result.Image.URL), nil
	}
	if strings.TrimSpace(result.URL) != "" {
		return strings.TrimSpace(result.URL), nil
	}
	return "", fmt.Errorf("fal.ai response contained no image URL")
}

func formatSocialReview(id int64, draft socialDraft) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📝 NOXIOAI social draft #%d — %s\n\n%s\n\nImage prompt: %s",
		id, draft.Platform, draft.Caption, draft.ImagePrompt)
	if draft.ImageURL != "" {
		b.WriteString("\nImage: " + draft.ImageURL)
	}
	if draft.Platform == "instagram" {
		fmt.Fprintf(&b, "\n\nApprove for manual Instagram posting:\njarvis social-approve %d\nReject: jarvis social-reject %d", id, id)
	} else {
		fmt.Fprintf(&b, "\n\nApprove and publish to Telegram:\njarvis social-approve %d\nReject: jarvis social-reject %d", id, id)
	}
	return b.String()
}

// ApproveSocialPost opens the human gate. Telegram drafts publish only after
// approval; Instagram drafts are deliberately left for official manual tools.
func ApproveSocialPost(ctx context.Context, db *sql.DB, id int64) (socialApproval, error) {
	if db == nil {
		return socialApproval{}, fmt.Errorf("Social agent: database is nil")
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE social_posts SET status = 'approved'
		WHERE id = $1 AND status = 'draft'`, id); err != nil {
		return socialApproval{}, fmt.Errorf("approve social post #%d: %w", id, err)
	}

	var platform, caption, imageURL, status string
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(platform,''), COALESCE(caption,''), COALESCE(image_url,''), COALESCE(status,'')
		FROM social_posts WHERE id = $1`, id).Scan(&platform, &caption, &imageURL, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return socialApproval{}, fmt.Errorf("social post #%d not found", id)
		}
		return socialApproval{}, fmt.Errorf("load social post #%d: %w", id, err)
	}
	result := socialApproval{ID: id, Platform: platform, Status: status}
	switch status {
	case "posted":
		result.AlreadyPosted = true
		return result, nil
	case "rejected":
		return socialApproval{}, fmt.Errorf("social post #%d is rejected", id)
	case "approved":
		// Continue to the platform-specific publishing step. This also lets an
		// owner retry a Telegram publish after a transient API failure.
	default:
		return socialApproval{}, fmt.Errorf("social post #%d has unsupported status %q", id, status)
	}

	if platform == "instagram" {
		return result, nil
	}
	channel := strings.TrimSpace(os.Getenv("JARVIS_SOCIAL_CHANNEL"))
	if channel == "" {
		return result, nil
	}
	token := strings.TrimSpace(os.Getenv("JARVIS_TELEGRAM_TOKEN"))
	if token == "" {
		return socialApproval{}, fmt.Errorf("publish social post #%d: JARVIS_TELEGRAM_TOKEN not set", id)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if err := publishSocialTelegram(ctx, client, token, channel, caption, imageURL); err != nil {
		return socialApproval{}, fmt.Errorf("publish social post #%d: %w", id, err)
	}
	update, err := db.ExecContext(ctx, `
		UPDATE social_posts SET status = 'posted', posted_at = now()
		WHERE id = $1 AND status = 'approved'`, id)
	if err != nil {
		return socialApproval{}, fmt.Errorf("mark social post #%d posted: %w", id, err)
	}
	rows, err := update.RowsAffected()
	if err != nil {
		return socialApproval{}, fmt.Errorf("confirm social post #%d status: %w", id, err)
	}
	if rows != 1 {
		return socialApproval{}, fmt.Errorf("social post #%d changed status before it could be marked posted", id)
	}
	result.Status = "posted"
	result.Published = true
	return result, nil
}

func RejectSocialPost(ctx context.Context, db *sql.DB, id int64) error {
	if db == nil {
		return fmt.Errorf("Social agent: database is nil")
	}
	var status string
	err := db.QueryRowContext(ctx, `
		UPDATE social_posts SET status = 'rejected'
		WHERE id = $1 AND status IN ('draft', 'approved')
		RETURNING status`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("social post #%d not found or already posted/rejected", id)
	}
	if err != nil {
		return fmt.Errorf("reject social post #%d: %w", id, err)
	}
	return nil
}

func publishSocialTelegram(ctx context.Context, client *http.Client, token, channel, caption, imageURL string) error {
	method := "sendMessage"
	values := url.Values{"chat_id": {channel}, "text": {caption}}
	if imageURL != "" {
		method = "sendPhoto"
		values = url.Values{"chat_id": {channel}, "photo": {imageURL}, "caption": {caption}}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+token+"/"+method, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Telegram response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram %s: %s", method, result.Description)
	}
	return nil
}
