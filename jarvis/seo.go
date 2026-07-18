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
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	seoDefaultSite       = "sc-domain:noxioai.com"
	seoGuardMessage      = "SEO agent: set JARVIS_GSC_SA_JSON (Google service-account key) to enable"
	gscReadOnlyScope     = "https://www.googleapis.com/auth/webmasters.readonly"
	gscSearchAnalytics   = "https://searchconsole.googleapis.com/webmasters/v3/sites/%s/searchAnalytics/query"
	seoHighImpressions   = 100
	seoLowCTR            = 0.02
	seoSearchRowLimit    = 100
	seoTelegramMaxRunes  = 4000
	seoBrainMaxInputRows = 100
)

// gscRow is the Search Analytics API's row shape. Clicks and impressions are
// doubles in Google's schema even though their aggregated values are counts.
type gscRow struct {
	Keys        []string `json:"keys"`
	Clicks      float64  `json:"clicks"`
	Impressions float64  `json:"impressions"`
	CTR         float64  `json:"ctr"`
	Position    float64  `json:"position"`
}

type gscQueryRequest struct {
	StartDate  string   `json:"startDate"`
	EndDate    string   `json:"endDate"`
	Dimensions []string `json:"dimensions"`
	RowLimit   int      `json:"rowLimit"`
}

type seoOpportunities struct {
	OnePushQueries []gscRow `json:"one_push_queries"`
	LowCTRPages    []gscRow `json:"low_ctr_pages"`
}

type seoBrainOpportunity struct {
	Type           string `json:"type"`
	Target         string `json:"target"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

type seoBrainReport struct {
	Summary          string                `json:"summary"`
	TopOpportunities []seoBrainOpportunity `json:"top_opportunities"`
	Recommendations  []string              `json:"recommendations"`
	BlogDraft        seoBrainBlogDraft     `json:"blog_draft"`
}

type seoBrainBlogDraft struct {
	Topic             string   `json:"topic"`
	TargetKeyword     string   `json:"target_keyword"`
	PrimaryLanguage   string   `json:"primary_language"`
	BilingualApproach string   `json:"bilingual_approach"`
	Outline           []string `json:"outline"`
}

type seoTotals struct {
	Clicks      int
	Impressions int
	AvgPosition float64
}

const seoSystemPrompt = `You are the autonomous SEO analyst for NOXIOAI (noxioai.com).
Use only the supplied Google Search Console data. Be concrete, evidence-led, and concise.
The site's primary language contexts are English (EN) and Persian (FA).
You create recommendations and a blog-topic/outline DRAFT only. Never claim to publish, schedule, or modify the site.`

// RunSEO performs one weekly SEO cycle. A server timer should invoke
// `jarvis seo`; this function deliberately has no publishing capability.
func RunSEO(ctx context.Context, db *sql.DB) error {
	keyPath, enabled, err := seoServiceAccountPath()
	if err != nil {
		return err
	}
	if !enabled {
		log.Print(seoGuardMessage)
		return nil
	}
	if db == nil {
		return fmt.Errorf("SEO agent: database is nil")
	}

	keyJSON, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Print(seoGuardMessage)
		return nil
	}
	if err != nil {
		return fmt.Errorf("SEO agent: read Google service-account key: %w", err)
	}
	jwtConfig, err := google.JWTConfigFromJSON(keyJSON, gscReadOnlyScope)
	if err != nil {
		return fmt.Errorf("SEO agent: parse Google service-account key: %w", err)
	}
	token, err := jwtConfig.TokenSource(ctx).Token()
	if err != nil {
		return fmt.Errorf("SEO agent: get Google access token: %w", err)
	}
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	client.Timeout = 45 * time.Second

	site := strings.TrimSpace(envOr("JARVIS_GSC_SITE", seoDefaultSite))
	end := time.Now().UTC().AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -27)
	startDate, endDate := start.Format("2006-01-02"), end.Format("2006-01-02")
	period := startDate + ".." + endDate

	queryRows, err := fetchGSCRows(ctx, client, site, "query", startDate, endDate)
	if err != nil {
		return fmt.Errorf("SEO agent: fetch query rows: %w", err)
	}
	pageRows, err := fetchGSCRows(ctx, client, site, "page", startDate, endDate)
	if err != nil {
		return fmt.Errorf("SEO agent: fetch page rows: %w", err)
	}

	opportunities := filterSEOOpportunities(queryRows, pageRows)
	report, err := analyzeSEO(ctx, NewBrainFromEnv(), period, queryRows, pageRows, opportunities)
	if err != nil {
		return fmt.Errorf("SEO agent: analyze with Brain: %w", err)
	}

	totalRows := pageRows
	if len(totalRows) == 0 {
		totalRows = queryRows
	}
	totals := summarizeGSCRows(totalRows)
	analysis := formatSEOAnalysis(report)
	blogDraft := formatSEOBlogDraft(report.BlogDraft)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO seo_reports (period, clicks, impressions, avg_position, analysis, blog_draft)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		period, totals.Clicks, totals.Impressions, totals.AvgPosition, analysis, blogDraft); err != nil {
		return fmt.Errorf("SEO agent: store report: %w", err)
	}

	if err := SendTelegram(formatSEOTelegram(period, totals, report)); err != nil {
		return fmt.Errorf("SEO agent: send Telegram summary: %w", err)
	}
	log.Printf("SEO agent: report stored and delivered for %s", period)
	return nil
}

// seoServiceAccountPath distinguishes the safe "not configured" state from
// genuine filesystem errors. Both the CLI and RunSEO use it so the guard runs
// before any DB or network dependency is touched.
func seoServiceAccountPath() (path string, enabled bool, err error) {
	path = strings.TrimSpace(os.Getenv("JARVIS_GSC_SA_JSON"))
	if path == "" {
		return "", false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("SEO agent: inspect Google service-account key: %w", err)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("SEO agent: Google service-account key path is a directory: %s", path)
	}
	return path, true, nil
}

func fetchGSCRows(ctx context.Context, client *http.Client, site, dimension, startDate, endDate string) ([]gscRow, error) {
	body, err := json.Marshal(gscQueryRequest{
		StartDate:  startDate,
		EndDate:    endDate,
		Dimensions: []string{dimension},
		RowLimit:   seoSearchRowLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	endpoint := fmt.Sprintf(gscSearchAnalytics, url.PathEscape(site))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Search Console returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	return parseGSCRows(responseBody)
}

// parseGSCRows is intentionally pure so response-shape handling is testable
// without Google credentials or a database.
func parseGSCRows(data []byte) ([]gscRow, error) {
	var response struct {
		Rows []gscRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Search Analytics response: %w", err)
	}
	return response.Rows, nil
}

// filterSEOOpportunities is the deterministic first pass before the Brain.
// "One push" positions are inclusive 5..15. A page is considered high-
// impression/low-CTR at 100+ impressions and below 2% CTR.
func filterSEOOpportunities(queryRows, pageRows []gscRow) seoOpportunities {
	var opportunities seoOpportunities
	for _, row := range queryRows {
		if len(row.Keys) > 0 && row.Position >= 5 && row.Position <= 15 {
			opportunities.OnePushQueries = append(opportunities.OnePushQueries, row)
		}
	}
	for _, row := range pageRows {
		if len(row.Keys) > 0 && row.Impressions >= seoHighImpressions && row.CTR < seoLowCTR {
			opportunities.LowCTRPages = append(opportunities.LowCTRPages, row)
		}
	}
	sortRowsByImpressions(opportunities.OnePushQueries)
	sortRowsByImpressions(opportunities.LowCTRPages)
	return opportunities
}

func sortRowsByImpressions(rows []gscRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Impressions == rows[j].Impressions {
			return rows[i].Clicks > rows[j].Clicks
		}
		return rows[i].Impressions > rows[j].Impressions
	})
}

func summarizeGSCRows(rows []gscRow) seoTotals {
	var clicks, impressions, weightedPosition float64
	for _, row := range rows {
		clicks += row.Clicks
		impressions += row.Impressions
		weightedPosition += row.Position * row.Impressions
	}
	var average float64
	if impressions > 0 {
		average = weightedPosition / impressions
	}
	return seoTotals{
		Clicks:      int(math.Round(clicks)),
		Impressions: int(math.Round(impressions)),
		AvgPosition: average,
	}
}

func analyzeSEO(ctx context.Context, brain *Brain, period string, queryRows, pageRows []gscRow, opportunities seoOpportunities) (*seoBrainReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input := struct {
		Period        string           `json:"period"`
		QueryRows     []gscRow         `json:"query_rows"`
		PageRows      []gscRow         `json:"page_rows"`
		PreFiltered   seoOpportunities `json:"pre_filtered_candidates"`
		LanguageNotes string           `json:"language_notes"`
	}{
		Period:        period,
		QueryRows:     capGSCRows(queryRows),
		PageRows:      capGSCRows(pageRows),
		PreFiltered:   opportunities,
		LanguageNotes: "EN and FA are the site's primary language contexts",
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf(`Analyze this 28-day Google Search Console dataset.

Prioritize:
1. Queries at average position 5-15 that are one push from page-one visibility.
2. Pages with high impressions but low CTR where title/meta changes could help.
3. Practical content or on-page fixes grounded in the supplied metrics.

Choose exactly ONE draft blog topic and outline for the strongest opportunity. Pick EN or FA as the primary language from the evidence, and explain how to adapt it for the other primary language. This is a draft only; do not publish or suggest automatic publishing.

Return ONLY valid JSON with this exact shape:
{"summary":"","top_opportunities":[{"type":"query|page","target":"","evidence":"metrics in one sentence","recommendation":"specific action"}],"recommendations":[""],"blog_draft":{"topic":"","target_keyword":"","primary_language":"EN|FA","bilingual_approach":"","outline":[""]}}

Return the best three opportunities when the data supports three.

GSC data:
%s`, data)
	messages := []Message{
		{Role: "system", Content: seoSystemPrompt},
		{Role: "user", Content: prompt},
	}
	out, err := brain.Chat(messages, nil)
	if err != nil {
		return nil, err
	}
	report, parseErr := parseSEOBrainReport(out)
	if parseErr == nil {
		return report, nil
	}

	out, err = brain.Chat(append(messages,
		Message{Role: "assistant", Content: out},
		Message{Role: "user", Content: "Invalid structured response: " + parseErr.Error() + ". Reply again with ONLY valid JSON in the required shape."},
	), nil)
	if err != nil {
		return nil, err
	}
	return parseSEOBrainReport(out)
}

func capGSCRows(rows []gscRow) []gscRow {
	if len(rows) <= seoBrainMaxInputRows {
		return rows
	}
	return rows[:seoBrainMaxInputRows]
}

func parseSEOBrainReport(out string) (*seoBrainReport, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object found")
	}
	var report seoBrainReport
	if err := json.Unmarshal([]byte(out[start:end+1]), &report); err != nil {
		return nil, fmt.Errorf("decode report JSON: %w", err)
	}
	if strings.TrimSpace(report.Summary) == "" {
		return nil, fmt.Errorf("summary is empty")
	}
	if len(report.TopOpportunities) == 0 {
		return nil, fmt.Errorf("top_opportunities is empty")
	}
	if strings.TrimSpace(report.BlogDraft.Topic) == "" || len(report.BlogDraft.Outline) == 0 {
		return nil, fmt.Errorf("blog topic or outline is empty")
	}
	return &report, nil
}

func formatSEOAnalysis(report *seoBrainReport) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(report.Summary))
	b.WriteString("\n\nTop opportunities:\n")
	for i, opportunity := range report.TopOpportunities {
		fmt.Fprintf(&b, "%d. [%s] %s — %s — %s\n", i+1, opportunity.Type,
			strings.TrimSpace(opportunity.Target), strings.TrimSpace(opportunity.Evidence),
			strings.TrimSpace(opportunity.Recommendation))
	}
	if len(report.Recommendations) > 0 {
		b.WriteString("\nRecommendations:\n")
		for _, recommendation := range report.Recommendations {
			b.WriteString("- " + strings.TrimSpace(recommendation) + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func formatSEOBlogDraft(draft seoBrainBlogDraft) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topic: %s\nTarget keyword: %s\nPrimary language: %s\nBilingual approach: %s\n\nOutline:\n",
		strings.TrimSpace(draft.Topic), strings.TrimSpace(draft.TargetKeyword),
		strings.TrimSpace(draft.PrimaryLanguage), strings.TrimSpace(draft.BilingualApproach))
	for i, section := range draft.Outline {
		fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(section))
	}
	return strings.TrimSpace(b.String())
}

func formatSEOTelegram(period string, totals seoTotals, report *seoBrainReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔎 NOXIOAI weekly SEO draft — %s\n%d clicks · %d impressions · avg position %.1f\n\nTop opportunities:\n",
		period, totals.Clicks, totals.Impressions, totals.AvgPosition)
	limit := len(report.TopOpportunities)
	if limit > 3 {
		limit = 3
	}
	for i := 0; i < limit; i++ {
		opportunity := report.TopOpportunities[i]
		fmt.Fprintf(&b, "%d. %s — %s\n", i+1, seoSingleLine(opportunity.Target), seoSingleLine(opportunity.Recommendation))
	}
	fmt.Fprintf(&b, "\nBlog topic (%s): %s\n\nDraft only — nothing was published.",
		strings.TrimSpace(report.BlogDraft.PrimaryLanguage), seoSingleLine(report.BlogDraft.Topic))
	return truncateSEOTelegram(b.String())
}

func seoSingleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func truncateSEOTelegram(text string) string {
	runes := []rune(text)
	if len(runes) <= seoTelegramMaxRunes {
		return text
	}
	return strings.TrimSpace(string(runes[:seoTelegramMaxRunes-1])) + "…"
}
