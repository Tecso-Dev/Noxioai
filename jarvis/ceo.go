package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// CEO mode — JARVIS may end an admin-chat reply with command lines that
// dispatch the same agent runs the HUD's action buttons trigger. The
// whitelist below is the entire vocabulary: approve/send/reject/delete are
// not in it, not as aliases, so the human approval gate HERALD/social/MADUSA
// enforce in code can never be reached from here.

type ceoVerb string

const (
	verbOracle       ceoVerb = "ORACLE"
	verbAtlas        ceoVerb = "ATLAS"
	verbBrief        ceoVerb = "BRIEF"
	verbInbox        ceoVerb = "INBOX"
	verbPixel        ceoVerb = "PIXEL"
	verbCaleb        ceoVerb = "CALEB"
	verbMadusaCycle  ceoVerb = "MADUSA_CYCLE"
	verbMadusaMap    ceoVerb = "MADUSA_MAP"
	verbMadusaStatus ceoVerb = "MADUSA_STATUS"
	verbHealth       ceoVerb = "HEALTH"
)

// ceoWhitelist is the complete set of dispatchable verbs. Nothing outside
// this map ever executes — see TestWhitelistExcludesOutboundVerbs.
var ceoWhitelist = map[ceoVerb]bool{
	verbOracle: true, verbAtlas: true, verbBrief: true, verbInbox: true,
	verbPixel: true, verbCaleb: true, verbMadusaCycle: true, verbMadusaMap: true,
	verbMadusaStatus: true, verbHealth: true,
}

type ceoCommand struct {
	Verb ceoVerb
	Arg  string
}

// ceoCmdLineRe matches a WHOLE trimmed line and nothing else — text before
// or after "[[CMD: ...]]" (mid-sentence, quoted, fenced) fails the anchor
// and is left as plain prose. This is also what decides what gets hidden
// from the browser/persisted chat, independent of whitelist validity.
var ceoCmdLineRe = regexp.MustCompile(`^\[\[CMD:\s*(\S+)(?:\s+(.+?))?\s*\]\]$`)

// parseCEOCommands extracts up to 2 whitelisted commands from a full model
// reply. Anything beyond 2 is ignored and logged, never executed. Malformed
// or non-whitelisted lines (including case variants of banned verbs) are
// silently dropped — a bad command is never surfaced as a user-facing error.
func parseCEOCommands(reply string) []ceoCommand {
	var cmds []ceoCommand
	truncated := 0
	for _, raw := range strings.Split(reply, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		m := ceoCmdLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if len(cmds) >= 2 {
			truncated++
			continue
		}
		cmd, ok := validateCEOCommand(strings.ToUpper(m[1]), m[2])
		if !ok {
			log.Printf("ceo: rejected command line %q", line)
			continue
		}
		cmds = append(cmds, cmd)
	}
	if truncated > 0 {
		log.Printf("ceo: %d extra command(s) beyond the 2-per-reply cap ignored", truncated)
	}
	return cmds
}

// validateCEOCommand checks the verb is whitelisted and its argument shape
// is sane. Newlines can never actually reach here (the caller already split
// on "\n" before matching a line), so the strip below is defense in depth.
func validateCEOCommand(verb, arg string) (ceoCommand, bool) {
	v := ceoVerb(verb)
	if !ceoWhitelist[v] {
		return ceoCommand{}, false
	}
	arg = strings.TrimSpace(strings.ReplaceAll(arg, "\n", " "))
	switch v {
	case verbOracle:
		if arg == "" || len(arg) > 120 {
			return ceoCommand{}, false
		}
	case verbAtlas, verbPixel:
		id, err := strconv.Atoi(arg)
		if err != nil || id <= 0 {
			return ceoCommand{}, false
		}
		arg = strconv.Itoa(id)
	default: // BRIEF, INBOX, CALEB, MADUSA_*, HEALTH take no argument
		if arg != "" {
			return ceoCommand{}, false
		}
	}
	return ceoCommand{Verb: v, Arg: arg}, true
}

// stripCEOCommandLines removes every "[[CMD: ...]]" line (whitelisted or
// not) from text bound for the browser or chat_messages, leaving the
// model's natural-language sentence intact.
func stripCEOCommandLines(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		if ceoCmdLineRe.MatchString(strings.TrimSpace(raw)) {
			continue
		}
		out = append(out, raw)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// sseLineFilter buffers streamed tokens to whole lines so a "[[CMD: ...]]"
// line can be recognized and swallowed before it ever reaches the browser.
// ponytail: trades the token-by-token typewriter feel for line-sized
// chunks; upgrade to a bounded lookahead buffer if that reads as choppy.
type sseLineFilter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	buf     strings.Builder
}

func newSSELineFilter(w http.ResponseWriter, flusher http.Flusher) *sseLineFilter {
	return &sseLineFilter{w: w, flusher: flusher}
}

func (f *sseLineFilter) Token(tok string) {
	f.buf.WriteString(tok)
	for {
		s := f.buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		line, rest := s[:idx], s[idx+1:]
		f.buf.Reset()
		f.buf.WriteString(rest)
		f.emit(line, true)
	}
}

// Flush emits whatever partial line is left unterminated at stream end.
func (f *sseLineFilter) Flush() {
	if f.buf.Len() == 0 {
		return
	}
	f.emit(f.buf.String(), false)
	f.buf.Reset()
}

func (f *sseLineFilter) emit(line string, newline bool) {
	if ceoCmdLineRe.MatchString(strings.TrimSpace(line)) {
		return
	}
	if newline {
		line += "\n"
	}
	if line == "" {
		return
	}
	data, _ := json.Marshal(map[string]string{"token": line})
	fmt.Fprintf(f.w, "data: %s\n\n", data)
	f.flusher.Flush()
}

// writeSSEToken sends one more browser-visible line after streaming has
// finished (used for the "⚙ dispatched: ..." notice).
func writeSSEToken(w http.ResponseWriter, flusher http.Flusher, text string) {
	data, _ := json.Marshal(map[string]string{"token": text})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// ceoBusy/ceoActivity replicate hud.go's busySet+activity run pattern for
// CEO-dispatched agents. hud.go's own instances are private to
// registerHUD's closure (out of the write set for this ticket), so this is
// a separate feed rather than a shared one — see task report.
var ceoBusy = &busySet{m: map[string]bool{}}
var ceoActivity = &activity{}

// ceoDispatch runs fn in a goroutine on a background (request-independent)
// context, marking busy and logging an activity line exactly like hud.go's
// runAgent — the chat reply never blocks on agent completion.
func ceoDispatch(agent, startMsg string, fn func(ctx context.Context) (string, error)) {
	ceoActivity.add("%s", startMsg)
	ceoBusy.set(agent, true)
	go func() {
		defer ceoBusy.set(agent, false)
		out, err := fn(context.Background())
		if err != nil {
			ceoActivity.add("✗ %s: %v", agent, err)
			return
		}
		ceoActivity.add("✓ %s: %s", agent, out)
	}()
}

// runCEOCommands dispatches each parsed command, one goroutine each, using
// the exact same run functions the HUD's action buttons call.
func runCEOCommands(db *sql.DB, brain *Brain, ownerID int64, cmds []ceoCommand) {
	for _, cmd := range cmds {
		cmd := cmd
		switch cmd.Verb {
		case verbOracle:
			ceoDispatch("ORACLE", "🔎 CEO dispatch: ORACLE hunting "+cmd.Arg, func(ctx context.Context) (string, error) {
				res, err := (&Oracle{Brain: brain, DB: db, OwnerID: ownerID}).Run(ctx, Task{Agent: "oracle", Input: cmd.Arg})
				return res.Output, err
			})
		case verbAtlas:
			ceoDispatch("ATLAS", "✍️ CEO dispatch: ATLAS drafting for lead "+cmd.Arg, func(ctx context.Context) (string, error) {
				res, err := (&Atlas{Brain: brain, DB: db, OwnerID: ownerID}).Run(ctx, Task{Agent: "atlas", Input: cmd.Arg})
				return res.Output, err
			})
		case verbPixel:
			ceoDispatch("PIXEL", "🎨 CEO dispatch: PIXEL reviewing lead "+cmd.Arg, func(ctx context.Context) (string, error) {
				id, _ := strconv.ParseInt(cmd.Arg, 10, 64)
				return RunPixel(ctx, db, ownerID, brain, id)
			})
		case verbCaleb:
			ceoDispatch("CALEB", "📈 CEO dispatch: CALEB drafting marketing memo", func(ctx context.Context) (string, error) {
				return RunCaleb(ctx, db, ownerID, brain)
			})
		case verbBrief:
			ceoDispatch("FRIDAY", "📨 CEO dispatch: briefing requested", func(ctx context.Context) (string, error) {
				return "delivered", RunBrief(ctx, db, ownerID, brain)
			})
		case verbInbox:
			ceoDispatch("HERALD", "📬 CEO dispatch: HERALD checking inbox", func(ctx context.Context) (string, error) {
				n, err := CheckInbox(ctx, db, ownerID)
				return fmt.Sprintf("%d replies processed", n), err
			})
		case verbMadusaCycle:
			ceoDispatch("MADUSA", "🎬 CEO dispatch: MADUSA cycle", func(ctx context.Context) (string, error) {
				return "cycle complete", RunMadusaCycle(ctx, db, brain, ownerID)
			})
		case verbMadusaMap:
			ceoDispatch("MADUSA", "🗺️ CEO dispatch: MADUSA map", func(ctx context.Context) (string, error) {
				return "map re-sent", MadusaMap(ctx, db)
			})
		case verbMadusaStatus:
			ceoDispatch("MADUSA", "📊 CEO dispatch: MADUSA status", func(ctx context.Context) (string, error) {
				return MadusaStatus(ctx, db)
			})
		case verbHealth:
			ceoDispatch("SYSTEM", "🩺 CEO dispatch: health check", func(ctx context.Context) (string, error) {
				return renderHealth(collectSystemStatus(ctx, db)), nil
			})
		}
	}
}

// ceoDispatchSummary renders the "⚙ dispatched: ..." SSE notice text.
func ceoDispatchSummary(cmds []ceoCommand) string {
	parts := make([]string, len(cmds))
	for i, c := range cmds {
		if c.Arg == "" {
			parts[i] = string(c.Verb)
		} else {
			parts[i] = string(c.Verb) + " " + c.Arg
		}
	}
	return "⚙ dispatched: " + strings.Join(parts, ", ")
}

// ceoSystemPromptSection is appended to the admin-chat system prompt only
// (never the CLI REPL or tenant/support bots) — CEO mode is an admin-console
// capability.
const ceoSystemPromptSection = `

## CEO mode
You are also Sobhan's executive coordinator / chief of staff for NOXIOAI. You
may dispatch background agent runs by ending your reply with command lines,
one per line, in this EXACT syntax:

[[CMD: <VERB> <args>]]

Whitelisted verbs (nothing else exists, there is no other syntax):
- ORACLE <niche text> — market research & lead scoring
- ATLAS <lead id> — drafts outreach copy (stored UNAPPROVED — Sobhan reviews before anything sends)
- BRIEF — FRIDAY's daily briefing
- INBOX — HERALD checks for and processes replies
- PIXEL <lead id> — design/motion critique
- CALEB — marketing strategy memo
- MADUSA_CYCLE — MADUSA trend-scout + content cycle
- MADUSA_MAP — MADUSA's opportunity map
- MADUSA_STATUS — MADUSA's current status
- HEALTH — system health snapshot

Rules:
- Dispatch only when Sobhan explicitly asks for action, or it obviously and
  directly serves what he just asked for.
- Always accompany a command with a natural-language sentence announcing
  what you are dispatching and why, e.g. "Dispatching ORACLE now, Sir." The
  command line itself is stripped before you're shown to him — the sentence
  is what he actually reads.
- Emit at most 2 command lines per reply.
- You can NEVER approve, send, reject, or delete anything — those verbs do
  not exist in your vocabulary. Outreach, social posts, and MADUSA content
  are always sent by Sobhan's own hand from the approval gate. If asked to
  send or approve something, tell him plainly that's his call, not yours.
- Answer in whichever language he used — English or Persian (فارسی).

Worked examples:
User: "Find me some fintech leads in Berlin"
JARVIS: "On it, Sir — dispatching ORACLE against the Berlin fintech market now.
[[CMD: ORACLE fintech companies in Berlin]]"

User: "Draft outreach for lead 42 and give me today's brief"
JARVIS: "Right away — ATLAS on lead 42, and I'll have FRIDAY pull together
today's briefing alongside it.
[[CMD: ATLAS 42]]
[[CMD: BRIEF]]"
`
