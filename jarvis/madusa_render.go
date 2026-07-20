package main

// MADUSA render worker — rents one Vultr GPU instance per batch, renders
// every packed post's storyboard with LTX-2.3 (via /opt/madusa/render.sh on
// a preinstalled snapshot), delivers the result to Telegram, and destroys
// the instance. The instance lifecycle is the money path: create → render →
// deliver → destroy, with an orphan reconcile pass and a hard wall-clock
// deadline so nothing bills forever by accident.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	madusaVultrGuard = "MADUSA RENDER: set JARVIS_VULTR_KEY to enable rendering"
	madusaVultrAPI   = "https://api.vultr.com/v2"
	madusaRenderTag  = "madusa-render"

	madusaSSHPollInterval = 20 * time.Second
	madusaSSHPollTimeout  = 15 * time.Minute

	// madusaTelegramVideoLimit is Telegram's hard cap for bot-uploaded video.
	madusaTelegramVideoLimit = 50 << 20
)

// ── env ──────────────────────────────────────────────────────────────────

func madusaVultrKey() string { return strings.TrimSpace(os.Getenv("JARVIS_VULTR_KEY")) }
func madusaRegion() string   { return envOr("JARVIS_VULTR_REGION", "ewr") }
func madusaSSHKeyPath() string {
	if v := strings.TrimSpace(os.Getenv("JARVIS_MADUSA_SSH_KEY")); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ssh", "id_ed25519")
	}
	return "/root/.ssh/id_ed25519"
}
func madusaOutDir() string { return envOr("JARVIS_MADUSA_OUT_DIR", "/var/lib/jarvis/madusa") }

// madusaTruncate cuts s to at most n runes — byte-slicing would split
// multi-byte UTF-8 (Persian captions) and produce invalid text.
func madusaTruncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func madusaMaxHours() float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv("JARVIS_MADUSA_MAX_HOURS")), 64)
	if err != nil || f <= 0 {
		return 3
	}
	return f
}

// ── pure math (unit-tested, no I/O) ────────────────────────────────────────

// madusaRenderDeadline is the absolute wall-clock deadline for a render
// batch that started at start and is capped at maxHours. maxHours <= 0 is
// guarded to the 3h default so a bad env value never disables the cap.
func madusaRenderDeadline(start time.Time, maxHours float64) time.Time {
	if maxHours <= 0 {
		maxHours = 3
	}
	return start.Add(time.Duration(maxHours * float64(time.Hour)))
}

// madusaElapsedHours is the wall-clock cost basis recorded on madusa_renders.
func madusaElapsedHours(start, end time.Time) float64 {
	return end.Sub(start).Hours()
}

// ── Vultr API v2 (plain net/http, no SDK) ───────────────────────────────────

type vultrPlan struct {
	ID          string  `json:"id"`
	MonthlyCost float64 `json:"monthly_cost"`
}

type vultrPlansResponse struct {
	Plans []vultrPlan `json:"plans"`
}

// madusaPickGPUPlan picks the cheapest plan whose id contains "l40s" from a
// GET /v2/plans?type=vcg response body.
func madusaPickGPUPlan(plansJSON []byte) (string, error) {
	var parsed vultrPlansResponse
	if err := json.Unmarshal(plansJSON, &parsed); err != nil {
		return "", fmt.Errorf("decode vultr plans: %w", err)
	}
	best := ""
	bestCost := math.MaxFloat64
	for _, p := range parsed.Plans {
		if !strings.Contains(p.ID, "l40s") {
			continue
		}
		if p.MonthlyCost < bestCost {
			bestCost = p.MonthlyCost
			best = p.ID
		}
	}
	if best == "" {
		return "", fmt.Errorf("no l40s GPU plan found in vultr plans response")
	}
	return best, nil
}

type vultrInstance struct {
	ID          string `json:"id"`
	MainIP      string `json:"main_ip"`
	Status      string `json:"status"`
	Label       string `json:"label"`
	DateCreated string `json:"date_created"`
}

type vultrInstanceResponse struct {
	Instance vultrInstance `json:"instance"`
}

type vultrInstanceListResponse struct {
	Instances []vultrInstance `json:"instances"`
}

// madusaParseInstance extracts id/ip/status from a GET or POST
// /v2/instances(/{id}) response body.
func madusaParseInstance(body []byte) (id, ip, status string, err error) {
	var parsed vultrInstanceResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", "", fmt.Errorf("decode vultr instance: %w", err)
	}
	if parsed.Instance.ID == "" {
		return "", "", "", fmt.Errorf("vultr instance response missing id")
	}
	return parsed.Instance.ID, parsed.Instance.MainIP, parsed.Instance.Status, nil
}

// madusaInstanceCreatedAt derives an instance's creation time from its
// "madusa-render-<unix>" label, falling back to date_created. Returns
// ok = false if neither yields a usable timestamp.
func madusaInstanceCreatedAt(inst vultrInstance) (time.Time, bool) {
	const labelPrefix = madusaRenderTag + "-"
	if strings.HasPrefix(inst.Label, labelPrefix) {
		if ts, err := strconv.ParseInt(strings.TrimPrefix(inst.Label, labelPrefix), 10, 64); err == nil {
			return time.Unix(ts, 0), true
		}
	}
	if inst.DateCreated != "" {
		if t, err := time.Parse(time.RFC3339, inst.DateCreated); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// madusaOrphanIDs returns the ids of tagged instances older than maxAge, from
// a GET /v2/instances?tag=madusa-render response body. An instance whose age
// cannot be determined is left alone rather than destroyed blind.
func madusaOrphanIDs(listJSON []byte, now time.Time, maxAge time.Duration) []string {
	var parsed vultrInstanceListResponse
	if err := json.Unmarshal(listJSON, &parsed); err != nil {
		return nil
	}
	var out []string
	for _, inst := range parsed.Instances {
		created, ok := madusaInstanceCreatedAt(inst)
		if !ok {
			continue
		}
		if now.Sub(created) > maxAge {
			out = append(out, inst.ID)
		}
	}
	return out
}

func madusaVultrRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, madusaVultrAPI+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+madusaVultrKey())
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, 0, err
	}
	return b, resp.StatusCode, nil
}

func madusaResolveGPUPlan(ctx context.Context) (string, error) {
	if plan := strings.TrimSpace(os.Getenv("JARVIS_VULTR_GPU_PLAN")); plan != "" {
		return plan, nil
	}
	body, status, err := madusaVultrRequest(ctx, http.MethodGet, "/plans?type=vcg", nil)
	if err != nil {
		return "", fmt.Errorf("vultr list plans: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("vultr list plans: status %d: %s", status, strings.TrimSpace(string(body)))
	}
	return madusaPickGPUPlan(body)
}

type vultrCreateInstanceRequest struct {
	Region     string   `json:"region"`
	Plan       string   `json:"plan"`
	SnapshotID string   `json:"snapshot_id"`
	Label      string   `json:"label"`
	Tag        string   `json:"tag"`
	SSHKeyIDs  []string `json:"sshkey_id,omitempty"`
}

func madusaCreateInstance(ctx context.Context, plan string) (id, label string, err error) {
	label = fmt.Sprintf("%s-%d", madusaRenderTag, time.Now().Unix())
	reqBody := vultrCreateInstanceRequest{
		Region:     madusaRegion(),
		Plan:       plan,
		SnapshotID: strings.TrimSpace(os.Getenv("JARVIS_VULTR_SNAPSHOT_ID")),
		Label:      label,
		Tag:        madusaRenderTag,
	}
	if sk := strings.TrimSpace(os.Getenv("JARVIS_VULTR_SSHKEY_ID")); sk != "" {
		reqBody.SSHKeyIDs = []string{sk}
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}
	body, status, err := madusaVultrRequest(ctx, http.MethodPost, "/instances", bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return "", "", fmt.Errorf("vultr create instance: status %d: %s", status, strings.TrimSpace(string(body)))
	}
	instID, _, _, err := madusaParseInstance(body)
	if err != nil {
		return "", "", err
	}
	return instID, label, nil
}

// madusaDestroyInstance issues DELETE /v2/instances/{id} then polls GET
// until the instance is gone (404) or 10 attempts are exhausted.
func madusaDestroyInstance(ctx context.Context, id string) error {
	_, status, err := madusaVultrRequest(ctx, http.MethodDelete, "/instances/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("vultr destroy instance %s: status %d", id, status)
	}
	for i := 0; i < 10; i++ {
		_, status, err := madusaVultrRequest(ctx, http.MethodGet, "/instances/"+id, nil)
		if err == nil && status == http.StatusNotFound {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("instance %s still alive after destroy + 10 poll attempts", id)
}

func madusaListRenderInstances(ctx context.Context) ([]byte, error) {
	body, status, err := madusaVultrRequest(ctx, http.MethodGet, "/instances?tag="+madusaRenderTag, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("vultr list instances: status %d: %s", status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// madusaReconcileOrphans destroys any madusa-render-tagged instance older
// than the max-hours budget, alerting the owner per destroyed orphan. Runs
// unconditionally at the top of every MadusaRender call, even when the
// render queue is empty — a stray instance must never survive on cost alone.
func madusaReconcileOrphans(ctx context.Context, db *sql.DB) error {
	body, err := madusaListRenderInstances(ctx)
	if err != nil {
		return fmt.Errorf("list instances for orphan reconcile: %w", err)
	}
	ids := madusaOrphanIDs(body, time.Now(), time.Duration(madusaMaxHours()*float64(time.Hour)))
	for _, id := range ids {
		if err := madusaDestroyInstance(ctx, id); err != nil {
			log.Printf("MADUSA RENDER: destroy orphan %s: %v", id, err)
			continue
		}
		msg := fmt.Sprintf("MADUSA RENDER: destroyed orphan GPU instance %s (older than %.1fh budget)", id, madusaMaxHours())
		log.Print(msg)
		if err := SendTelegram(msg); err != nil {
			log.Printf("MADUSA RENDER: alert for orphan %s: %v", id, err)
		}
	}
	return nil
}

// ── ssh/scp (shell out — no new go.mod deps) ────────────────────────────────

func madusaSSHArgs(extra ...string) []string {
	return append([]string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-i", madusaSSHKeyPath(),
	}, extra...)
}

func madusaSSHReachable(ctx context.Context, ip string) bool {
	args := madusaSSHArgs("root@"+ip, "true")
	return exec.CommandContext(ctx, "ssh", args...).Run() == nil
}

func madusaRunRenderScript(ctx context.Context, ip string, postID int64, remoteSB, remoteOutDir string) error {
	args := madusaSSHArgs("root@"+ip, fmt.Sprintf("/opt/madusa/render.sh %s %s", remoteSB, remoteOutDir))
	out, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("render.sh for post #%d: %w: %s", postID, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func madusaSCPUpload(ctx context.Context, ip, localPath, remotePath string) error {
	args := madusaSSHArgs(localPath, "root@"+ip+":"+remotePath)
	out, err := exec.CommandContext(ctx, "scp", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp upload %s: %w: %s", localPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func madusaSCPDownload(ctx context.Context, ip, remotePath, localPath string) error {
	args := madusaSSHArgs("root@"+ip+":"+remotePath, localPath)
	out, err := exec.CommandContext(ctx, "scp", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp download %s: %w: %s", remotePath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// madusaWaitForInstance polls until the instance is active with an assigned
// IP AND ssh-reachable, or the 15-minute hard cap elapses.
func madusaWaitForInstance(ctx context.Context, id string) (string, error) {
	deadline := time.Now().Add(madusaSSHPollTimeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		body, status, err := madusaVultrRequest(ctx, http.MethodGet, "/instances/"+id, nil)
		if err == nil && status == http.StatusOK {
			if _, ip, instStatus, perr := madusaParseInstance(body); perr == nil && instStatus == "active" && ip != "" {
				if madusaSSHReachable(ctx, ip) {
					return ip, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(madusaSSHPollInterval):
		}
	}
	return "", fmt.Errorf("instance %s did not become active+ssh-reachable within %s", id, madusaSSHPollTimeout)
}

// ── Telegram video delivery ──────────────────────────────────────────────

// madusaSendVideo posts a video file to the owner's Telegram chat via
// sendVideo. Files over Telegram's 50MB bot-upload cap fall back to a text
// message naming the file's server path instead.
func madusaSendVideo(path, caption string) error {
	token := os.Getenv("JARVIS_TELEGRAM_TOKEN")
	chat := os.Getenv("JARVIS_TELEGRAM_CHAT")
	if token == "" || chat == "" {
		return fmt.Errorf("JARVIS_TELEGRAM_TOKEN / JARVIS_TELEGRAM_CHAT not set (jarvis/.env)")
	}
	caption = madusaTruncate(caption, 1024)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat video %s: %w", path, err)
	}
	if info.Size() > madusaTelegramVideoLimit {
		return SendTelegram(fmt.Sprintf("MADUSA: video too large for Telegram upload (%d bytes) — file on server at %s\n\n%s",
			info.Size(), path, caption))
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("chat_id", chat); err != nil {
		return err
	}
	if err := w.WriteField("caption", caption); err != nil {
		return err
	}
	part, err := w.CreateFormFile("video", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+token+"/sendVideo", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("telegram sendVideo %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// ── orchestration ────────────────────────────────────────────────────────

type madusaRenderQueueItem struct {
	ID     int64
	Status string
}

func madusaLoadRenderQueue(ctx context.Context, db *sql.DB) ([]madusaRenderQueueItem, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, status FROM madusa_posts
		WHERE status IN ('approved', 'packed')
		ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []madusaRenderQueueItem
	for rows.Next() {
		var it madusaRenderQueueItem
		if err := rows.Scan(&it.ID, &it.Status); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func madusaMarkPostFailed(ctx context.Context, db *sql.DB, id int64, reason string) {
	if _, err := db.ExecContext(ctx, `UPDATE madusa_posts SET status = 'failed' WHERE id = $1`, id); err != nil {
		log.Printf("MADUSA RENDER: mark post #%d failed: %v", id, err)
	}
	msg := fmt.Sprintf("MADUSA RENDER: post #%d failed — %s", id, reason)
	log.Print(msg)
	if err := SendTelegram(msg); err != nil {
		log.Printf("MADUSA RENDER: alert for post #%d: %v", id, err)
	}
}

// madusaRenderOnePost renders, delivers, and marks delivered a single packed
// post on an already-reachable instance. Any failure here is scoped to this
// post — the caller marks it failed and moves on to the next.
func madusaRenderOnePost(ctx context.Context, db *sql.DB, ip, outDir string, postID int64) error {
	post, pkg, err := madusaLoadPackedPost(ctx, db, postID)
	if err != nil {
		return err
	}

	sbBytes, err := json.Marshal(pkg.Storyboard)
	if err != nil {
		return fmt.Errorf("marshal storyboard: %w", err)
	}
	tmpSB, err := os.CreateTemp("", fmt.Sprintf("madusa-sb-%d-*.json", postID))
	if err != nil {
		return fmt.Errorf("create temp storyboard: %w", err)
	}
	defer os.Remove(tmpSB.Name())
	if _, err := tmpSB.Write(sbBytes); err != nil {
		tmpSB.Close()
		return fmt.Errorf("write temp storyboard: %w", err)
	}
	tmpSB.Close()

	remoteSB := fmt.Sprintf("/tmp/sb-%d.json", postID)
	remoteOutDir := fmt.Sprintf("/tmp/out-%d", postID)
	if err := madusaSCPUpload(ctx, ip, tmpSB.Name(), remoteSB); err != nil {
		return fmt.Errorf("upload storyboard: %w", err)
	}
	if err := madusaRunRenderScript(ctx, ip, postID, remoteSB, remoteOutDir); err != nil {
		return err
	}

	localDir := filepath.Join(outDir, strconv.FormatInt(postID, 10))
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("create local out dir: %w", err)
	}
	localVideo := filepath.Join(localDir, "final.mp4")
	localThumb := filepath.Join(localDir, "thumb.jpg")
	if err := madusaSCPDownload(ctx, ip, remoteOutDir+"/final.mp4", localVideo); err != nil {
		return fmt.Errorf("download video: %w", err)
	}
	if err := madusaSCPDownload(ctx, ip, remoteOutDir+"/thumb.jpg", localThumb); err != nil {
		log.Printf("MADUSA RENDER: download thumb for post #%d: %v", postID, err)
	}

	shortCaption := madusaTruncate(pkg.CaptionFA, 200)
	if err := madusaSendVideo(localVideo, shortCaption); err != nil {
		log.Printf("MADUSA RENDER: telegram sendVideo for post #%d: %v", postID, err)
	}
	if err := SendTelegram(formatMadusaDelivery(post, pkg)); err != nil {
		log.Printf("MADUSA RENDER: telegram delivery report for post #%d: %v", postID, err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE madusa_posts SET status = 'delivered', delivered_at = now(), video_url = $1 WHERE id = $2`,
		localVideo, postID); err != nil {
		return fmt.Errorf("mark post delivered: %w", err)
	}
	return nil
}

// MadusaRender is the sole entry point for the render worker. It reconciles
// orphaned GPU instances first (always), then — if there is a queue — packs
// any not-yet-packed posts, rents exactly one instance for the whole batch,
// renders each post on it, and destroys the instance on every exit path
// (success, per-post failure, timeout, or panic).
func MadusaRender(ctx context.Context, db *sql.DB, brain *Brain) (err error) {
	if db == nil {
		return fmt.Errorf("MADUSA RENDER: database is nil")
	}
	if madusaVultrKey() == "" {
		log.Print(madusaVultrGuard)
		return nil
	}
	if strings.TrimSpace(os.Getenv("JARVIS_VULTR_SNAPSHOT_ID")) == "" {
		return fmt.Errorf("MADUSA RENDER: JARVIS_VULTR_SNAPSHOT_ID is required")
	}

	if rerr := madusaReconcileOrphans(ctx, db); rerr != nil {
		log.Printf("MADUSA RENDER: orphan reconcile: %v", rerr)
	}

	deadline := madusaRenderDeadline(time.Now(), madusaMaxHours())
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	queue, err := madusaLoadRenderQueue(ctx, db)
	if err != nil {
		return fmt.Errorf("MADUSA RENDER: load queue: %w", err)
	}
	if len(queue) == 0 {
		log.Print("MADUSA RENDER: queue empty")
		return nil
	}

	var packed []int64
	for _, item := range queue {
		if item.Status == "approved" {
			if brain == nil {
				madusaMarkPostFailed(ctx, db, item.ID, "brain unavailable for packaging")
				continue
			}
			if perr := MadusaPack(ctx, db, brain, item.ID); perr != nil {
				madusaMarkPostFailed(ctx, db, item.ID, "pack failed: "+perr.Error())
				continue
			}
		}
		packed = append(packed, item.ID)
	}
	if len(packed) == 0 {
		log.Print("MADUSA RENDER: nothing packed successfully, nothing to render")
		return nil
	}

	plan, err := madusaResolveGPUPlan(ctx)
	if err != nil {
		return fmt.Errorf("MADUSA RENDER: resolve GPU plan: %w", err)
	}

	batchStart := time.Now()
	instID, _, err := madusaCreateInstance(ctx, plan)
	if err != nil {
		return fmt.Errorf("MADUSA RENDER: create instance: %w", err)
	}

	var renderRowID int64

	// From the moment we have an instance id, it MUST be destroyed on every
	// exit path — success, per-post failure, timeout, or panic.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("MADUSA RENDER: recovered panic during render batch: %v", r)
			if err == nil {
				err = fmt.Errorf("MADUSA RENDER: panic: %v", r)
			}
		}
		destroyCtx, dcancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer dcancel()
		finalStatus := "destroyed"
		if derr := madusaDestroyInstance(destroyCtx, instID); derr != nil {
			finalStatus = "failed"
			log.Printf("MADUSA RENDER: CRITICAL failed to destroy instance %s: %v", instID, derr)
			_ = SendTelegram(fmt.Sprintf("🚨 MADUSA RENDER CRITICAL: failed to destroy GPU instance %s — manual cleanup required now. %v", instID, derr))
		}
		hours := madusaElapsedHours(batchStart, time.Now())
		if _, uerr := db.ExecContext(context.Background(), `
			UPDATE madusa_renders SET status = $1, cost_hours = $2, finished_at = now() WHERE id = $3`,
			finalStatus, hours, renderRowID); uerr != nil {
			log.Printf("MADUSA RENDER: update render row %d on teardown: %v", renderRowID, uerr)
		}
	}()

	if qerr := db.QueryRowContext(ctx, `
		INSERT INTO madusa_renders (instance_id, status) VALUES ($1, 'creating') RETURNING id`,
		instID).Scan(&renderRowID); qerr != nil {
		log.Printf("MADUSA RENDER: insert render row for instance %s: %v", instID, qerr)
	}

	ip, werr := madusaWaitForInstance(ctx, instID)
	if werr != nil {
		_ = SendTelegram(fmt.Sprintf("MADUSA RENDER: instance %s never became reachable: %v", instID, werr))
		return fmt.Errorf("MADUSA RENDER: wait for instance: %w", werr)
	}
	if _, uerr := db.ExecContext(ctx, `UPDATE madusa_renders SET instance_ip = $1, status = 'rendering' WHERE id = $2`, ip, renderRowID); uerr != nil {
		log.Printf("MADUSA RENDER: record instance ip: %v", uerr)
	}

	outDir := madusaOutDir()
	for _, postID := range packed {
		if ctx.Err() != nil {
			madusaMarkPostFailed(ctx, db, postID, "batch deadline exceeded")
			continue
		}
		if perr := madusaRenderOnePost(ctx, db, ip, outDir, postID); perr != nil {
			madusaMarkPostFailed(ctx, db, postID, perr.Error())
		}
	}
	return nil
}
