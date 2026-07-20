package main

// MADUSA — trend scout + content machine (SPEC: approved 2026-07-20). It
// watches AI-automation YouTube creators plus Reddit/HN chatter, scores
// momentum, asks the Brain what will go viral next, and proposes short-form
// video ideas (the "MAP") that a human approves before anything renders.

import (
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
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	madusaYouTubeGuard = "MADUSA: set JARVIS_YOUTUBE_KEY to enable YouTube ingest (Reddit/HN still run)"
	madusaMapFooter    = "Approve: jarvis madusa approve <id> · Reject: jarvis madusa reject <id>"

	// madusaVelocitySmall is the boundary between a "small" and a "large"
	// trend velocity for stage classification — see madusaTrendStage.
	madusaVelocitySmall = 0.5

	madusaRedditUserAgent = "noxioai-madusa/1.0"
)

// Best-effort seed list of AI-automation YouTube creators. Runtime handle
// resolution fails soft per handle (log.Printf + continue) so one dead
// handle never blocks the rest of the cycle.
var madusaSeedCreators = []string{
	"mreflow", "theAIGRID", "matthew_berman", "WesRoth", "DavidOndrej",
	"LiamOttley", "nateherk", "ColeMedin", "AIJasonZ", "futurepedia",
	"SkillLeapAI", "vrsen", "jonocatliff", "rileybrownai", "aiadvantage",
}

var madusaSubreddits = []string{
	"ArtificialInteligence", "ChatGPT", "automation", "aiagents", "SaaS", "smallbusiness", "artificial",
}

// ── seeding ──────────────────────────────────────────────────────────────

// MadusaSeedCreators inserts the seed list, ignoring creators already known.
func MadusaSeedCreators(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	for _, handle := range madusaSeedCreators {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO madusa_creators (platform, handle, added_by)
			VALUES ('youtube', $1, 'seed')
			ON CONFLICT (handle) DO NOTHING`, handle); err != nil {
			return fmt.Errorf("MADUSA: seed creator %s: %w", handle, err)
		}
	}
	return nil
}

// ── YouTube ingest (plain net/http, no google client libs) ────────────────
//
// Quota budget: channels.list and playlistItems.list and videos.list each
// cost ~1 unit per call; batching ids 50-at-a-time keeps a full cycle over
// ~15 seed creators well under 200 units. search.list (100 units/call) is
// NEVER called in this cycle.

type ytChannelsResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title string `json:"title"`
		} `json:"snippet"`
		Statistics struct {
			SubscriberCount string `json:"subscriberCount"`
			ViewCount       string `json:"viewCount"`
			VideoCount      string `json:"videoCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func madusaResolveChannel(ctx context.Context, client *http.Client, apiKey, handle string) (channelID, title string, err error) {
	u := "https://www.googleapis.com/youtube/v3/channels?part=id,snippet,statistics,contentDetails&forHandle=" +
		url.QueryEscape(handle) + "&key=" + url.QueryEscape(apiKey)
	body, status, err := madusaGet(ctx, client, u)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("youtube channels.list (forHandle): %s", strings.TrimSpace(string(body)))
	}
	var parsed ytChannelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("decode youtube channels response: %w", err)
	}
	if len(parsed.Items) == 0 {
		return "", "", fmt.Errorf("no channel found for handle %s", handle)
	}
	return parsed.Items[0].ID, parsed.Items[0].Snippet.Title, nil
}

type madusaChannelStat struct {
	id         string
	subs       int64
	views      int64
	videoCount int
}

func madusaFetchChannelStats(ctx context.Context, client *http.Client, apiKey string, ids []string) ([]madusaChannelStat, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := "https://www.googleapis.com/youtube/v3/channels?part=statistics&id=" +
		url.QueryEscape(strings.Join(ids, ",")) + "&key=" + url.QueryEscape(apiKey)
	body, status, err := madusaGet(ctx, client, u)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("youtube channels.list (stats): %s", strings.TrimSpace(string(body)))
	}
	var parsed ytChannelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode youtube stats response: %w", err)
	}
	out := make([]madusaChannelStat, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		subs, _ := strconv.ParseInt(item.Statistics.SubscriberCount, 10, 64)
		views, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
		videoCount, _ := strconv.Atoi(item.Statistics.VideoCount)
		out = append(out, madusaChannelStat{id: item.ID, subs: subs, views: views, videoCount: videoCount})
	}
	return out, nil
}

// madusaUploadsPlaylistID derives the uploads-playlist id from a channel id
// (the well-known "UC" → "UU" prefix swap), avoiding a search.list call.
func madusaUploadsPlaylistID(channelID string) string {
	if strings.HasPrefix(channelID, "UC") {
		return "UU" + channelID[2:]
	}
	return channelID
}

type ytPlaylistItemsResponse struct {
	Items []struct {
		ContentDetails struct {
			VideoID string `json:"videoId"`
		} `json:"contentDetails"`
	} `json:"items"`
}

func madusaRecentVideoIDs(ctx context.Context, client *http.Client, apiKey, channelID string) ([]string, error) {
	playlistID := madusaUploadsPlaylistID(channelID)
	u := "https://www.googleapis.com/youtube/v3/playlistItems?part=contentDetails&maxResults=10&playlistId=" +
		url.QueryEscape(playlistID) + "&key=" + url.QueryEscape(apiKey)
	body, status, err := madusaGet(ctx, client, u)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("youtube playlistItems.list: %s", strings.TrimSpace(string(body)))
	}
	var parsed ytPlaylistItemsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode youtube playlistItems response: %w", err)
	}
	ids := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if item.ContentDetails.VideoID != "" {
			ids = append(ids, item.ContentDetails.VideoID)
		}
	}
	return ids, nil
}

type ytVideosResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			PublishedAt string `json:"publishedAt"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func madusaFetchVideos(ctx context.Context, client *http.Client, apiKey string, ids []string) (ytVideosResponse, error) {
	var out ytVideosResponse
	if len(ids) == 0 {
		return out, nil
	}
	u := "https://www.googleapis.com/youtube/v3/videos?part=snippet,statistics,contentDetails&id=" +
		url.QueryEscape(strings.Join(ids, ",")) + "&key=" + url.QueryEscape(apiKey)
	body, status, err := madusaGet(ctx, client, u)
	if err != nil {
		return out, err
	}
	if status != http.StatusOK {
		return out, fmt.Errorf("youtube videos.list: %s", strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode youtube videos response: %w", err)
	}
	return out, nil
}

func madusaGet(ctx context.Context, client *http.Client, u string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "noxioai-madusa/1.0")
	resp, err := client.Do(req)
	if err != nil {
		// *url.Error embeds the full request URL; strip the query string so
		// key= params (JARVIS_YOUTUBE_KEY) can never reach the logs.
		if ue, ok := err.(*url.Error); ok {
			if pu, perr := url.Parse(u); perr == nil {
				pu.RawQuery = ""
				err = fmt.Errorf("GET %s: %v", pu.String(), ue.Err)
			}
		}
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

type madusaCreatorRow struct {
	id        int64
	handle    string
	channelID string
}

// MadusaIngestYouTube resolves handles, refreshes daily channel snapshots,
// and refreshes each creator's recent videos. Missing JARVI_YOUTUBE_KEY skips
// YouTube entirely (guard message logged) without failing the caller.
func MadusaIngestYouTube(ctx context.Context, db *sql.DB) error {
	apiKey := strings.TrimSpace(os.Getenv("JARVIS_YOUTUBE_KEY"))
	if apiKey == "" {
		log.Print(madusaYouTubeGuard)
		return nil
	}
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	client := &http.Client{Timeout: 20 * time.Second}

	rows, err := db.QueryContext(ctx, `SELECT id, handle, COALESCE(channel_id,'') FROM madusa_creators WHERE active = true`)
	if err != nil {
		return fmt.Errorf("MADUSA: load creators: %w", err)
	}
	var creators []madusaCreatorRow
	for rows.Next() {
		var c madusaCreatorRow
		if err := rows.Scan(&c.id, &c.handle, &c.channelID); err != nil {
			rows.Close()
			return fmt.Errorf("MADUSA: scan creator row: %w", err)
		}
		creators = append(creators, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("MADUSA: iterate creators: %w", err)
	}

	for i := range creators {
		if creators[i].channelID != "" {
			continue
		}
		channelID, title, err := madusaResolveChannel(ctx, client, apiKey, creators[i].handle)
		if err != nil {
			log.Printf("MADUSA: resolve handle %s: %v", creators[i].handle, err)
			continue
		}
		creators[i].channelID = channelID
		if _, err := db.ExecContext(ctx, `UPDATE madusa_creators SET channel_id = $1, title = $2 WHERE id = $3`,
			channelID, title, creators[i].id); err != nil {
			log.Printf("MADUSA: store channel id for %s: %v", creators[i].handle, err)
		}
	}

	byChannelID := make(map[string]int64, len(creators))
	ids := make([]string, 0, len(creators))
	for _, c := range creators {
		if c.channelID != "" {
			byChannelID[c.channelID] = c.id
			ids = append(ids, c.channelID)
		}
	}
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		stats, err := madusaFetchChannelStats(ctx, client, apiKey, ids[start:end])
		if err != nil {
			log.Printf("MADUSA: fetch channel stats: %v", err)
			continue
		}
		for _, s := range stats {
			creatorID, ok := byChannelID[s.id]
			if !ok {
				continue
			}
			if _, err := db.ExecContext(ctx, `
				INSERT INTO madusa_snapshots (creator_id, subs, views, video_count)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (creator_id, day) DO UPDATE SET subs = $2, views = $3, video_count = $4`,
				creatorID, s.subs, s.views, s.videoCount); err != nil {
				log.Printf("MADUSA: upsert snapshot for creator %d: %v", creatorID, err)
			}
		}
	}

	for _, c := range creators {
		if c.channelID == "" {
			continue
		}
		if err := madusaRefreshCreatorVideos(ctx, db, client, apiKey, c.id, c.channelID); err != nil {
			log.Printf("MADUSA: refresh videos for %s: %v", c.handle, err)
		}
	}
	return nil
}

func madusaRefreshCreatorVideos(ctx context.Context, db *sql.DB, client *http.Client, apiKey string, creatorID int64, channelID string) error {
	videoIDs, err := madusaRecentVideoIDs(ctx, client, apiKey, channelID)
	if err != nil {
		return err
	}
	for start := 0; start < len(videoIDs); start += 50 {
		end := start + 50
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		videos, err := madusaFetchVideos(ctx, client, apiKey, videoIDs[start:end])
		if err != nil {
			return err
		}
		for _, v := range videos.Items {
			views, _ := strconv.ParseInt(v.Statistics.ViewCount, 10, 64)
			likes, _ := strconv.ParseInt(v.Statistics.LikeCount, 10, 64)
			comments, _ := strconv.ParseInt(v.Statistics.CommentCount, 10, 64)
			publishedAt, _ := time.Parse(time.RFC3339, v.Snippet.PublishedAt)
			if _, err := db.ExecContext(ctx, `
				INSERT INTO madusa_videos (creator_id, video_id, title, published_at, views, likes, comments)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (video_id) DO UPDATE SET
					views_prev   = madusa_videos.views,
					fetched_prev = madusa_videos.fetched_at,
					views        = $5,
					likes        = $6,
					comments     = $7,
					fetched_at   = now()`,
				creatorID, v.ID, v.Snippet.Title, publishedAt, views, likes, comments); err != nil {
				log.Printf("MADUSA: upsert video %s: %v", v.ID, err)
			}
		}
	}
	return nil
}

// ── Reddit + HN ingest ─────────────────────────────────────────────────────

type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				Title       string `json:"title"`
				Permalink   string `json:"permalink"`
				Score       int    `json:"score"`
				NumComments int    `json:"num_comments"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// MadusaIngestReddit fetches r/<sub>/hot.json for the watched subreddits.
// Each subreddit fails soft so one blocked/renamed sub does not stop the rest.
func MadusaIngestReddit(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	for _, sub := range madusaSubreddits {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://www.reddit.com/r/"+sub+"/hot.json?limit=40", nil)
		if err != nil {
			log.Printf("MADUSA: reddit request for r/%s: %v", sub, err)
			continue
		}
		req.Header.Set("User-Agent", madusaRedditUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("MADUSA: reddit fetch r/%s: %v", sub, err)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			log.Printf("MADUSA: reddit read r/%s: %v", sub, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("MADUSA: reddit r/%s returned %s", sub, resp.Status)
			continue
		}
		var listing redditListing
		if err := json.Unmarshal(body, &listing); err != nil {
			log.Printf("MADUSA: decode reddit r/%s: %v", sub, err)
			continue
		}
		for _, child := range listing.Data.Children {
			link := "https://www.reddit.com" + child.Data.Permalink
			if err := madusaUpsertSignal(ctx, db, "reddit", child.Data.Title, link, child.Data.Score, child.Data.NumComments); err != nil {
				log.Printf("MADUSA: store reddit signal: %v", err)
			}
		}
	}
	return nil
}

type hnHit struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	ObjectID    string `json:"objectID"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
}

type hnResponse struct {
	Hits []hnHit `json:"hits"`
}

// MadusaIngestHN pulls the AI front-page and recent high-signal "AI agent"
// stories from HN Algolia. Each endpoint fails soft.
func MadusaIngestHN(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	endpoints := []string{
		"https://hn.algolia.com/api/v1/search?query=AI&tags=front_page",
		"https://hn.algolia.com/api/v1/search_by_date?query=%22AI%20agent%22&tags=story&numericFilters=points%3E20",
	}
	for _, u := range endpoints {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, status, err := madusaGet(ctx, client, u)
		if err != nil {
			log.Printf("MADUSA: hn fetch: %v", err)
			continue
		}
		if status != http.StatusOK {
			log.Printf("MADUSA: hn returned status %d", status)
			continue
		}
		var parsed hnResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			log.Printf("MADUSA: decode hn response: %v", err)
			continue
		}
		for _, hit := range parsed.Hits {
			link := hit.URL
			if link == "" {
				link = "https://news.ycombinator.com/item?id=" + hit.ObjectID
			}
			if err := madusaUpsertSignal(ctx, db, "hackernews", hit.Title, link, hit.Points, hit.NumComments); err != nil {
				log.Printf("MADUSA: store hn signal: %v", err)
			}
		}
	}
	return nil
}

func madusaUpsertSignal(ctx context.Context, db *sql.DB, source, title, link string, score, comments int) error {
	title = strings.TrimSpace(title)
	link = strings.TrimSpace(link)
	if title == "" || link == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO madusa_signals (source, title, url, score, comments)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (url) DO UPDATE SET
			score_prev   = madusa_signals.score,
			fetched_prev = madusa_signals.fetched_at,
			score        = $4,
			comments     = $5,
			fetched_at   = now()`,
		source, title, link, score, comments)
	return err
}

// ── pure math (unit-tested, no I/O) ────────────────────────────────────────

// madusaCreatorMomentum weights three signals into one score:
//   - relative subscriber growth rate, per day  (weight 0.4)
//   - relative view growth rate, per day        (weight 0.4)
//   - posting frequency vs a 3-posts/week baseline, capped at 2x (weight 0.2)
//
// Guards: days <= 0 is treated as 1 day; subsPrev/viewsPrev <= 0 zeroes the
// corresponding growth term instead of dividing by zero; postsRecent14d <= 0
// zeroes the posting-frequency term instead of going negative.
func madusaCreatorMomentum(subsPrev, subsCur, viewsPrev, viewsCur int64, days float64, postsRecent14d int) float64 {
	if days <= 0 {
		days = 1
	}
	var subGrowth float64
	if subsPrev > 0 {
		subGrowth = float64(subsCur-subsPrev) / float64(subsPrev) / days
	}
	var viewGrowth float64
	if viewsPrev > 0 {
		viewGrowth = float64(viewsCur-viewsPrev) / float64(viewsPrev) / days
	}
	const baselinePostsPerWeek = 3.0
	var postScore float64
	if postsRecent14d > 0 {
		postsPerWeek := float64(postsRecent14d) / 14.0 * 7.0
		postScore = postsPerWeek / baselinePostsPerWeek
		if postScore > 2 {
			postScore = 2 // cap: a runaway posting cadence should not dominate the score
		}
	}
	return subGrowth*0.4 + viewGrowth*0.4 + postScore*0.2
}

// madusaTrendStage classifies a trend's lifecycle stage from its velocity
// (score change per hour) and acceleration (change in velocity). Rules are
// applied in order, most specific first:
//
//  1. velocity <= 0                        → "dying"   (no longer growing, regardless of acceleration)
//  2. velocity >  0 && accel <  0           → "peaking" (still positive, but decelerating)
//  3. velocity >= madusaVelocitySmall (0.5) → "growing" (already a large velocity)
//  4. velocity >  0 && accel >  0           → "growing" (small but sustained acceleration)
//  5. otherwise                             → "emerging" (small, flat acceleration — an early signal)
func madusaTrendStage(velocity, accel float64) string {
	switch {
	case velocity <= 0:
		return "dying"
	case accel < 0:
		return "peaking"
	case velocity >= madusaVelocitySmall:
		return "growing"
	case accel > 0:
		return "growing"
	default:
		return "emerging"
	}
}

// madusaSignalVelocity guards a zero/negative time window by treating it as
// one hour, so a same-cycle re-fetch never divides by zero.
func madusaSignalVelocity(scorePrev, scoreCur int, hoursBetween float64) float64 {
	if hoursBetween <= 0 {
		hoursBetween = 1
	}
	return float64(scoreCur-scorePrev) / hoursBetween
}

// ── brain analysis ─────────────────────────────────────────────────────────

const madusaSystemPrompt = `You are MADUSA, NOXIOAI's trend-scout and content-machine agent for the AI-automation niche.
Your job: predict what will go viral next among AI-automation creators and communities, then propose short-form video ideas (a "MAP" — Map of Actionable Posts) that tie honestly back to NOXIOAI's offer.

NOXIOAI builds practical AI employees that answer customers and automate routine work for businesses (Persian-first ICP: Iranian business owners; also EU services). Never invent customers, testimonials, prices, results, percentages, or statistics. Do not make guarantees.

Reply with ONLY valid JSON, no prose, in this exact shape:
{"trends":[{"topic":"","stage":"","why":"","hook_patterns":""}],"map":[{"idea":"","hook":"","format":"","platforms":["reel","short"],"why_now":"","tie_to_noxioai":""}]}

Produce 3-5 map items. Every map item's "why_now" must reference specific evidence from the data given. Every map item's "tie_to_noxioai" must connect the trend to NOXIOAI's real offer honestly — no invented numbers, customers, or guarantees.
Every "hook" must name its pattern in parentheses — one of (curiosity gap), (pattern interrupt), (bold claim), (question), (number) — followed by the actual opening line to say on camera. Example: "(curiosity gap) Nobody tells you this about AI agents..."`

type madusaCreatorBrief struct {
	Handle       string
	Momentum     float64
	RecentTitles []string
}

type madusaSignalBrief struct {
	Source   string
	Title    string
	Velocity float64
}

type madusaTrendOut struct {
	Topic        string `json:"topic"`
	Stage        string `json:"stage"`
	Why          string `json:"why"`
	HookPatterns string `json:"hook_patterns"`
}

type madusaMapItem struct {
	Idea         string   `json:"idea"`
	Hook         string   `json:"hook"`
	Format       string   `json:"format"`
	Platforms    []string `json:"platforms"`
	WhyNow       string   `json:"why_now"`
	TieToNoxioai string   `json:"tie_to_noxioai"`
}

type madusaAnalysis struct {
	Trends []madusaTrendOut `json:"trends"`
	Map    []madusaMapItem  `json:"map"`
}

// madusaAnalyze compacts the top data into one prompt and makes exactly one
// Brain call (plus one retry if the reply fails to parse).
func madusaAnalyze(ctx context.Context, brain *Brain, creators []madusaCreatorBrief, signals []madusaSignalBrief, yesterdayTrends []string) (madusaAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return madusaAnalysis{}, err
	}
	var b strings.Builder
	b.WriteString("Top creators by momentum (handle, momentum score, recent video titles):\n")
	for _, c := range creators {
		fmt.Fprintf(&b, "- %s (momentum %.3f): %s\n", c.Handle, c.Momentum, strings.Join(c.RecentTitles, " | "))
	}
	b.WriteString("\nTop signals by velocity (source, title, velocity/hr):\n")
	for _, s := range signals {
		fmt.Fprintf(&b, "- [%s] %s (velocity %.2f/hr)\n", s.Source, s.Title, s.Velocity)
	}
	if len(yesterdayTrends) > 0 {
		b.WriteString("\nYesterday's trends (for continuity):\n- " + strings.Join(yesterdayTrends, "\n- "))
	}

	messages := []Message{
		{Role: "system", Content: madusaSystemPrompt},
		{Role: "user", Content: b.String()},
	}
	out, err := brain.Chat(messages, nil)
	if err != nil {
		return madusaAnalysis{}, err
	}
	analysis, parseErr := parseMadusaAnalysis(out)
	if parseErr == nil {
		return analysis, nil
	}

	if err := ctx.Err(); err != nil {
		return madusaAnalysis{}, err
	}
	out, err = brain.Chat(append(messages,
		Message{Role: "assistant", Content: out},
		Message{Role: "user", Content: "The response was invalid: " + parseErr.Error() + ". Reply again with ONLY valid JSON in the required shape."},
	), nil)
	if err != nil {
		return madusaAnalysis{}, err
	}
	return parseMadusaAnalysis(out)
}

// parseMadusaAnalysis is pure so model-response handling can be tested
// without credentials, network access, or a database.
func parseMadusaAnalysis(out string) (madusaAnalysis, error) {
	start := strings.IndexAny(out, "{[")
	if start < 0 {
		return madusaAnalysis{}, fmt.Errorf("no JSON found in MADUSA analysis")
	}
	var end int
	if out[start] == '{' {
		end = strings.LastIndex(out, "}")
	} else {
		end = strings.LastIndex(out, "]")
	}
	if end <= start {
		return madusaAnalysis{}, fmt.Errorf("no closing bracket found in MADUSA analysis")
	}
	var analysis madusaAnalysis
	if err := json.Unmarshal([]byte(out[start:end+1]), &analysis); err != nil {
		return madusaAnalysis{}, fmt.Errorf("decode MADUSA analysis JSON: %w", err)
	}
	if len(analysis.Map) == 0 {
		return madusaAnalysis{}, fmt.Errorf("MADUSA analysis produced no map items")
	}
	return analysis, nil
}

// ── MAP report + cycle orchestration ───────────────────────────────────────

type madusaPostRow struct {
	ID     int64
	Idea   string
	Hook   string
	Format string
	Stage  string
	WhyNow string
}

// formatMadusaMapReport is pure so it can be tested without a database or
// Telegram credentials.
func formatMadusaMapReport(rows []madusaPostRow) string {
	var b strings.Builder
	b.WriteString("🐍 MADUSA MAP\n\n")
	if len(rows) == 0 {
		b.WriteString("No proposed items.")
		return b.String()
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "#%d — %s\nHook: %s\nFormat: %s · Stage: %s\nWhy now: %s\n\n",
			r.ID, r.Idea, r.Hook, r.Format, r.Stage, r.WhyNow)
	}
	b.WriteString(madusaMapFooter)
	return b.String()
}

// RunMadusaCycle seeds creators if empty, ingests every source (each fails
// soft), scores momentum, asks the Brain for trends + MAP, persists both,
// and sends the MADUSA MAP report to the owner for approval.
func RunMadusaCycle(ctx context.Context, db *sql.DB, brain *Brain, ownerID int64) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM madusa_creators WHERE active = true`).Scan(&count); err != nil {
		return fmt.Errorf("MADUSA: count creators: %w", err)
	}
	if count == 0 {
		if err := MadusaSeedCreators(ctx, db); err != nil {
			return fmt.Errorf("MADUSA: seed creators: %w", err)
		}
	}

	if err := MadusaIngestYouTube(ctx, db); err != nil {
		log.Printf("MADUSA: youtube ingest: %v", err)
	}
	if err := MadusaIngestReddit(ctx, db); err != nil {
		log.Printf("MADUSA: reddit ingest: %v", err)
	}
	if err := MadusaIngestHN(ctx, db); err != nil {
		log.Printf("MADUSA: hn ingest: %v", err)
	}

	creators, err := madusaLoadTopCreators(ctx, db, 10)
	if err != nil {
		log.Printf("MADUSA: load top creators: %v", err)
	}
	signals, err := madusaLoadTopSignals(ctx, db, 20)
	if err != nil {
		log.Printf("MADUSA: load top signals: %v", err)
	}
	yesterdayTrends, err := madusaLoadYesterdayTrends(ctx, db)
	if err != nil {
		log.Printf("MADUSA: load yesterday trends: %v", err)
	}

	if brain == nil {
		return fmt.Errorf("MADUSA: brain is nil")
	}
	analysis, err := madusaAnalyze(ctx, brain, creators, signals, yesterdayTrends)
	if err != nil {
		log.Printf("MADUSA: brain analysis: %v", err)
		return nil // cycle continues without a MAP this time
	}

	for _, t := range analysis.Trends {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO madusa_trends (topic, stage, evidence, hook_patterns)
			VALUES ($1, $2, $3, $4)`,
			t.Topic, t.Stage, t.Why, t.HookPatterns); err != nil {
			log.Printf("MADUSA: store trend %q: %v", t.Topic, err)
		}
	}

	rows := make([]madusaPostRow, 0, len(analysis.Map))
	for _, item := range analysis.Map {
		var id int64
		if err := db.QueryRowContext(ctx, `
			INSERT INTO madusa_posts (idea, hook, format, status)
			VALUES ($1, $2, $3, 'proposed')
			RETURNING id`, item.Idea, item.Hook, item.Format).Scan(&id); err != nil {
			log.Printf("MADUSA: store map item %q: %v", item.Idea, err)
			continue
		}
		rows = append(rows, madusaPostRow{ID: id, Idea: item.Idea, Hook: item.Hook, Format: item.Format, WhyNow: item.WhyNow})
	}

	if err := SendTelegram(formatMadusaMapReport(rows)); err != nil {
		return fmt.Errorf("MADUSA: deliver MAP report: %w", err)
	}
	log.Printf("MADUSA: cycle complete, %d map items proposed", len(rows))
	return nil
}

// MadusaMap re-sends the currently proposed MAP items.
func MadusaMap(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT p.id, p.idea, COALESCE(p.hook,''), p.format, COALESCE(t.stage,'')
		FROM madusa_posts p
		LEFT JOIN madusa_trends t ON t.id = p.trend_id
		WHERE p.status = 'proposed'
		ORDER BY p.id DESC`)
	if err != nil {
		return fmt.Errorf("MADUSA: load proposed posts: %w", err)
	}
	defer rows.Close()
	var out []madusaPostRow
	for rows.Next() {
		var r madusaPostRow
		if err := rows.Scan(&r.ID, &r.Idea, &r.Hook, &r.Format, &r.Stage); err != nil {
			return fmt.Errorf("MADUSA: scan proposed post: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("MADUSA: iterate proposed posts: %w", err)
	}
	return SendTelegram(formatMadusaMapReport(out))
}

func madusaLoadTopSignals(ctx context.Context, db *sql.DB, limit int) ([]madusaSignalBrief, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source, title, COALESCE(score,0), COALESCE(score_prev,0), fetched_at, fetched_prev
		FROM madusa_signals
		ORDER BY fetched_at DESC
		LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var briefs []madusaSignalBrief
	for rows.Next() {
		var source, title string
		var score, scorePrev int
		var fetchedAt time.Time
		var fetchedPrev sql.NullTime
		if err := rows.Scan(&source, &title, &score, &scorePrev, &fetchedAt, &fetchedPrev); err != nil {
			return nil, err
		}
		hours := 1.0
		if fetchedPrev.Valid {
			hours = fetchedAt.Sub(fetchedPrev.Time).Hours()
		}
		briefs = append(briefs, madusaSignalBrief{
			Source: source, Title: title,
			Velocity: madusaSignalVelocity(scorePrev, score, hours),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(briefs, func(i, j int) bool { return briefs[i].Velocity > briefs[j].Velocity })
	if len(briefs) > limit {
		briefs = briefs[:limit]
	}
	return briefs, nil
}

func madusaLoadTopCreators(ctx context.Context, db *sql.DB, limit int) ([]madusaCreatorBrief, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.handle,
		       COALESCE(s1.subs,0), COALESCE(s0.subs,0),
		       COALESCE(s1.views,0), COALESCE(s0.views,0),
		       GREATEST(1, COALESCE(s1.day - s0.day, 1)) AS days,
		       (SELECT COUNT(*) FROM madusa_videos v WHERE v.creator_id = c.id AND v.published_at > now() - interval '14 days') AS posts14d
		FROM madusa_creators c
		LEFT JOIN LATERAL (
			SELECT subs, views, day FROM madusa_snapshots WHERE creator_id = c.id ORDER BY day DESC LIMIT 1
		) s1 ON true
		LEFT JOIN LATERAL (
			SELECT subs, views, day FROM madusa_snapshots WHERE creator_id = c.id ORDER BY day DESC OFFSET 1 LIMIT 1
		) s0 ON true
		WHERE c.active = true`)
	if err != nil {
		return nil, err
	}
	type row struct {
		id                  int64
		handle              string
		subsCur, subsPrev   int64
		viewsCur, viewsPrev int64
		days                float64
		posts14d            int
	}
	var loaded []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.handle, &r.subsCur, &r.subsPrev, &r.viewsCur, &r.viewsPrev, &r.days, &r.posts14d); err != nil {
			rows.Close()
			return nil, err
		}
		loaded = append(loaded, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	briefs := make([]madusaCreatorBrief, 0, len(loaded))
	for _, r := range loaded {
		momentum := madusaCreatorMomentum(r.subsPrev, r.subsCur, r.viewsPrev, r.viewsCur, r.days, r.posts14d)
		titles, err := madusaRecentVideoTitles(ctx, db, r.id, 3)
		if err != nil {
			log.Printf("MADUSA: load recent videos for %s: %v", r.handle, err)
		}
		briefs = append(briefs, madusaCreatorBrief{Handle: r.handle, Momentum: momentum, RecentTitles: titles})
	}
	sort.Slice(briefs, func(i, j int) bool { return briefs[i].Momentum > briefs[j].Momentum })
	if len(briefs) > limit {
		briefs = briefs[:limit]
	}
	return briefs, nil
}

func madusaRecentVideoTitles(ctx context.Context, db *sql.DB, creatorID int64, limit int) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT title FROM madusa_videos
		WHERE creator_id = $1
		ORDER BY published_at DESC NULLS LAST
		LIMIT $2`, creatorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var title sql.NullString
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		if title.Valid {
			titles = append(titles, title.String)
		}
	}
	return titles, rows.Err()
}

func madusaLoadYesterdayTrends(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT topic FROM madusa_trends WHERE day = current_date - interval '1 day'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

// ── approval + creator management ──────────────────────────────────────────

// MadusaApprove moves a proposed MAP item to approved. It errors if the post
// does not exist or is not currently proposed.
func MadusaApprove(ctx context.Context, db *sql.DB, id int64) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	var status string
	err := db.QueryRowContext(ctx, `
		UPDATE madusa_posts SET status = 'approved', approved_at = now()
		WHERE id = $1 AND status = 'proposed'
		RETURNING status`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("MADUSA: post #%d not found or not in proposed status", id)
	}
	if err != nil {
		return fmt.Errorf("MADUSA: approve post #%d: %w", id, err)
	}
	return nil
}

// MadusaReject rejects a proposed or approved MAP item.
func MadusaReject(ctx context.Context, db *sql.DB, id int64) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	var status string
	err := db.QueryRowContext(ctx, `
		UPDATE madusa_posts SET status = 'rejected'
		WHERE id = $1 AND status IN ('proposed', 'approved')
		RETURNING status`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("MADUSA: post #%d not found or already delivered/rejected", id)
	}
	if err != nil {
		return fmt.Errorf("MADUSA: reject post #%d: %w", id, err)
	}
	return nil
}

// MadusaStatus returns a one-screen text summary of the current MADUSA state.
func MadusaStatus(ctx context.Context, db *sql.DB) (string, error) {
	if db == nil {
		return "", fmt.Errorf("MADUSA: database is nil")
	}
	var creators int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM madusa_creators WHERE active = true`).Scan(&creators); err != nil {
		return "", fmt.Errorf("MADUSA: count creators: %w", err)
	}
	var lastCycle sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT MAX(created_at) FROM madusa_trends`).Scan(&lastCycle); err != nil {
		return "", fmt.Errorf("MADUSA: load last cycle time: %w", err)
	}

	counts := make(map[string]int)
	rows, err := db.QueryContext(ctx, `SELECT status, COUNT(*) FROM madusa_posts GROUP BY status`)
	if err != nil {
		return "", fmt.Errorf("MADUSA: count posts by status: %w", err)
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return "", fmt.Errorf("MADUSA: scan post status count: %w", err)
		}
		counts[status] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("MADUSA: iterate post status counts: %w", err)
	}

	last := "never"
	if lastCycle.Valid {
		last = lastCycle.Time.Format("2006-01-02 15:04 MST")
	}
	return fmt.Sprintf("MADUSA status\nActive creators: %d\nLast cycle: %s\nProposed: %d · Approved: %d · Rendering: %d · Delivered: %d",
		creators, last, counts["proposed"], counts["approved"], counts["rendering"], counts["delivered"]), nil
}

// MadusaCreatorAdd adds (or reactivates) a YouTube creator by handle.
func MadusaCreatorAdd(ctx context.Context, db *sql.DB, handle string) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return fmt.Errorf("MADUSA: handle is empty")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO madusa_creators (platform, handle, added_by)
		VALUES ('youtube', $1, 'owner')
		ON CONFLICT (handle) DO UPDATE SET active = true`, handle); err != nil {
		return fmt.Errorf("MADUSA: add creator %s: %w", handle, err)
	}
	return nil
}

// MadusaCreatorRemove deactivates a creator (rows are kept for history).
func MadusaCreatorRemove(ctx context.Context, db *sql.DB, handle string) error {
	if db == nil {
		return fmt.Errorf("MADUSA: database is nil")
	}
	if _, err := db.ExecContext(ctx, `UPDATE madusa_creators SET active = false WHERE handle = $1`,
		strings.TrimSpace(handle)); err != nil {
		return fmt.Errorf("MADUSA: remove creator %s: %w", handle, err)
	}
	return nil
}

// MadusaCreatorList returns a one-screen text list of every known creator.
func MadusaCreatorList(ctx context.Context, db *sql.DB) (string, error) {
	if db == nil {
		return "", fmt.Errorf("MADUSA: database is nil")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT handle, active, COALESCE(title,'') FROM madusa_creators ORDER BY handle`)
	if err != nil {
		return "", fmt.Errorf("MADUSA: load creators: %w", err)
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("MADUSA creators\n")
	for rows.Next() {
		var handle, title string
		var active bool
		if err := rows.Scan(&handle, &active, &title); err != nil {
			return "", fmt.Errorf("MADUSA: scan creator: %w", err)
		}
		mark := "✅"
		if !active {
			mark = "⏸"
		}
		fmt.Fprintf(&b, "%s %s %s\n", mark, handle, title)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("MADUSA: iterate creators: %w", err)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
