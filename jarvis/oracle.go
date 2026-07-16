package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ORACLE — market intelligence agent (SPEC Phase 2): search the niche,
// read each company's site, score it as a potential NOXIOAI client, store the lead.
type Oracle struct {
	Brain   *Brain
	DB      *sql.DB
	OwnerID int64
}

func (o *Oracle) Name() string { return "oracle" }

const oracleTarget = 12 // leads to store per run; stops early when the web runs dry

func (o *Oracle) Run(ctx context.Context, task Task) (Result, error) {
	niche := strings.TrimSpace(task.Input)
	if niche == "" {
		return Result{}, fmt.Errorf(`usage: jarvis oracle "real estate agencies in Warsaw"`)
	}
	fmt.Printf("🔎 ORACLE hunting: %s\n", niche)
	var candidates []candidate
	seen := map[string]bool{}
	for _, q := range o.searchQueries(niche) {
		found, err := searchWeb(ctx, q)
		if err != nil {
			fmt.Printf("  ✗ search %q: %v\n", q, err)
			continue
		}
		for _, c := range found {
			if !seen[c.Host] {
				seen[c.Host] = true
				candidates = append(candidates, c)
			}
		}
	}
	if len(candidates) == 0 {
		return Result{}, fmt.Errorf("no candidate sites found for %q", niche)
	}
	fmt.Printf("   %d candidate sites\n", len(candidates))

	stored := 0
	for _, cand := range candidates {
		if stored >= oracleTarget {
			break
		}
		page, emails, err := fetchText(ctx, cand.URL)
		if err != nil {
			fmt.Printf("  ✗ %-30s fetch: %v\n", cand.Host, err)
			continue
		}
		lead, err := o.extract(ctx, niche, cand, page)
		if err != nil {
			fmt.Printf("  ✗ %-30s extract: %v\n", cand.Host, err)
			continue
		}
		if lead.Skip {
			fmt.Printf("  – %-30s not a single company's site\n", cand.Host)
			continue
		}
		if err := o.store(ctx, cand, lead, niche, emails); err != nil {
			fmt.Printf("  ✗ %-30s store: %v\n", cand.Host, err)
			continue
		}
		stored++
		fmt.Printf("  ✓ %-30s %3d %-6s %s\n", cand.Host, lead.Score, tier(lead.Score), oneLine(lead.ObservedProblem, 60))
	}
	return Result{Output: fmt.Sprintf("%d leads stored for %q", stored, niche)}, nil
}

// --- web search ---

type candidate struct {
	URL  string
	Host string
}

func normalizeHost(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimPrefix(host, "www.")
	for _, prefix := range []string{"en.", "de.", "pl.", "fr."} {
		if strings.HasPrefix(host, prefix) {
			return strings.TrimPrefix(host, prefix)
		}
	}
	return host
}

// searchQueries asks the brain for query variants (one in the local language
// of the niche's location) so one run covers more ground than one DDG page.
func (o *Oracle) searchQueries(niche string) []string {
	prompt := fmt.Sprintf(`Give 3 web search queries to find INDIVIDUAL COMPANY WEBSITES for this niche: %q.
One query must be in the local language of the location if it is not English.
Reply with ONLY a JSON array of 3 strings.`, niche)
	out, err := o.Brain.Chat([]Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return []string{niche}
	}
	queries := parseFactArray(out) // tolerant JSON string-array parser from memory.go
	if len(queries) == 0 {
		return []string{niche}
	}
	return append([]string{niche}, queries...)
}

var resultLink = regexp.MustCompile(`class="result__a"[^>]+href="([^"]+)"`)

// ponytail: HTML-scraped DuckDuckGo search — brittle by nature; swap for a
// real search API (Serper/Brave, needs a key) when it breaks or quality matters.
func searchWeb(ctx context.Context, query string) ([]candidate, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://html.duckduckgo.com/html/?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return parseSearchResults(string(body)), nil
}

func parseSearchResults(page string) []candidate {
	var out []candidate
	seen := map[string]bool{}
	for _, m := range resultLink.FindAllStringSubmatch(page, -1) {
		u := html.UnescapeString(m[1])
		// DDG wraps targets: //duckduckgo.com/l/?uddg=<url-encoded target>&rut=…
		if p, err := url.Parse(u); err == nil {
			if real := p.Query().Get("uddg"); real != "" {
				u = real
			}
		}
		p, err := url.Parse(u)
		if err != nil || p.Host == "" || junkHost(p.Host) || seen[p.Host] {
			continue
		}
		seen[p.Host] = true
		out = append(out, candidate{URL: u, Host: p.Host})
	}
	return out
}

var junkHosts = []string{
	"duckduckgo.", "google.", "facebook.", "instagram.", "linkedin.",
	"youtube.", "wikipedia.", "yelp.", "tripadvisor.", "twitter.", "x.com",
}

func junkHost(h string) bool {
	h = strings.ToLower(h)
	for _, j := range junkHosts {
		if strings.Contains(h, j) {
			return true
		}
	}
	return false
}

// --- page fetch ---

const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace  = regexp.MustCompile(`\s+`)
	emailRE  = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.-]+\.[a-z]{2,}`)
)

func extractEmails(rawHTML string) []string {
	var emails []string
	seen := map[string]bool{}
	for _, email := range emailRE.FindAllString(rawHTML, -1) {
		email = strings.ToLower(strings.TrimSpace(email))
		email = strings.TrimPrefix(email, "%20") // URL-encoded leading space from mailto: hrefs
		if len(email) > 100 || seen[email] {
			continue
		}
		// encoding artifacts / obfuscation placeholders are not real addresses
		if strings.ContainsAny(email, " %<>()[]") {
			continue
		}
		local, domain, ok := strings.Cut(email, "@")
		if !ok {
			continue
		}
		if strings.HasSuffix(email, ".png") || strings.HasSuffix(email, ".jpg") ||
			strings.HasSuffix(email, ".jpeg") || strings.HasSuffix(email, ".gif") ||
			strings.HasSuffix(email, ".svg") || strings.HasSuffix(email, ".webp") {
			continue
		}
		junk := false
		for _, s := range []string{"noreply", "no-reply", "example", "sentry", "wixpress", "wix.com", "godaddy", "cloudflare", "protected", "schema.org", "w3.org", "googleapis", "gstatic", "jquery"} {
			if strings.Contains(local, s) || strings.Contains(domain, s) {
				junk = true
				break
			}
		}
		if junk {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
		if len(emails) == 5 {
			break
		}
	}
	return emails
}

func stripHTML(s string) string {
	s = reScript.ReplaceAllString(s, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

func fetchText(ctx context.Context, pageURL string) (text string, emails []string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 500<<10))
	if err != nil {
		return "", nil, err
	}
	raw := string(b)
	emails = extractEmails(raw)
	text = oneLine(stripHTML(raw), 4500)
	if len(text) < 200 {
		return "", nil, fmt.Errorf("no readable text (likely JS-only site — Playwright upgrade path)")
	}
	return text, emails, nil
}

// --- LLM extraction & scoring ---

type leadExtract struct {
	Skip            bool   `json:"skip"`
	Name            string `json:"name"`
	Industry        string `json:"industry"`
	Country         string `json:"country"`
	Notes           string `json:"notes"`
	Score           int    `json:"score"`
	Reasoning       string `json:"reasoning"`
	ObservedProblem string `json:"observed_problem"`
	SuggestedOffer  string `json:"suggested_offer"`
	Contacts        []struct {
		Name     string `json:"name"`
		Role     string `json:"role"`
		Email    string `json:"email"`
		Linkedin string `json:"linkedin"`
	} `json:"contacts"`
}

const extractLeadPrompt = `You are ORACLE, the market-intelligence agent of NOXIOAI — a software agency selling web development, e-commerce, AI automation and UI/UX design.

Analyze ONE company website and reply with ONLY a JSON object, no other text:
{"skip":false,"name":"","industry":"","country":"","notes":"2-3 sentence company summary","score":0,"reasoning":"2-4 sentences: why this score, citing specifics from the page","observed_problem":"single clearest weakness NOXIOAI could fix for them","suggested_offer":"concrete service to pitch","contacts":[{"name":"","role":"","email":"","linkedin":""}]}

Rules:
- If the page is NOT a single company's own website (directory, listicle, portal, marketplace, news, blog), reply {"skip":true}.
- score 0-100 = how promising a CLIENT this company is for a web/AI agency. Consider: website quality (a dated or weak site with a real business behind it = HIGH score), tech stack age, online activity, apparent size and budget.
- Only list contacts actually present in the text. Empty array is fine.

Searched niche: %s
Website URL: %s
%sPage text:
%s`

func (o *Oracle) extract(ctx context.Context, niche string, cand candidate, page string) (*leadExtract, error) {
	lessonsBlock := ""
	if lessons, _ := RecentLessons(ctx, o.DB, o.OwnerID, "oracle", 3); len(lessons) > 0 {
		lessonsBlock = "Problems seen at similar companies before (for context):\n- " + strings.Join(lessons, "\n- ") + "\n"
	}
	prompt := fmt.Sprintf(extractLeadPrompt, niche, cand.URL, lessonsBlock, page)
	out, err := o.Brain.Chat([]Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return nil, err
	}
	lead, perr := parseLeadJSON(out)
	if perr != nil { // one retry — models decorate JSON
		out, err = o.Brain.Chat([]Message{
			{Role: "user", Content: prompt},
			{Role: "assistant", Content: out},
			{Role: "user", Content: "Reply again with ONLY the valid JSON object."},
		}, nil)
		if err != nil {
			return nil, err
		}
		if lead, perr = parseLeadJSON(out); perr != nil {
			return nil, perr
		}
	}
	return lead, nil
}

func parseLeadJSON(out string) (*leadExtract, error) {
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object in model output")
	}
	var l leadExtract
	if err := json.Unmarshal([]byte(out[start:end+1]), &l); err != nil {
		return nil, err
	}
	if !l.Skip {
		if l.Score < 0 {
			l.Score = 0
		}
		if l.Score > 100 {
			l.Score = 100
		}
		if l.Name == "" || l.Reasoning == "" {
			return nil, fmt.Errorf("extraction missing name/reasoning")
		}
	}
	return &l, nil
}

func tier(score int) string {
	switch {
	case score >= 90:
		return "VIP"
	case score >= 70:
		return "HIGH"
	case score >= 40:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// --- persistence ---

func (o *Oracle) store(ctx context.Context, cand candidate, l *leadExtract, niche string, emails []string) error {
	website := "https://" + normalizeHost(cand.Host) // canonical per-company key
	companyID, err := UpsertCompany(ctx, o.DB, o.OwnerID, l.Name, website, l.Industry, l.Country, l.Notes)
	if err != nil {
		return err
	}
	if err := UpsertLead(ctx, o.DB, o.OwnerID, companyID, l.Score, tier(l.Score), l.Reasoning, l.ObservedProblem, l.SuggestedOffer); err != nil {
		return err
	}
	for _, c := range l.Contacts {
		if c.Name == "" && c.Email == "" && c.Linkedin == "" {
			continue
		}
		if err := AddContact(ctx, o.DB, o.OwnerID, companyID, c.Name, c.Role, c.Email, c.Linkedin); err != nil {
			return err
		}
	}
	for _, email := range emails {
		if email == "" {
			continue
		}
		if err := AddContact(ctx, o.DB, o.OwnerID, companyID, "", "", email, ""); err != nil {
			return err
		}
	}
	return AddExperience(ctx, o.DB, o.OwnerID, "oracle",
		fmt.Sprintf("niche: %s, site: %s", niche, cand.Host),
		fmt.Sprintf("scored %d (%s)", l.Score, tier(l.Score)),
		"lead stored", l.ObservedProblem)
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
