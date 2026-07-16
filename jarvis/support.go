package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const noxioSupportSystemPrompt = `You are NOXIOAI customer support. Answer concisely, helpfully, and in the customer's language.

NOXIOAI builds AI employees that automate business work including marketing, social media, customer support, and web/app development. They keep running while the customer sleeps.

Noxio Autopilot services:
- Starter Automation: €490 one-time for 1 automation, delivered in 7 days.
- Business Autopilot: €1490 one-time for 3 automations plus a Telegram briefing, delivered in 14 days.
- Autopilot Care: €75/month for hosting and upkeep.

Contact: hi@noxioai.com
Website: noxioai.com

Do not invent details. If you do not know the answer, the customer needs a human, or the request needs a custom quote, say that a human will follow up.`

const (
	supportPollTimeout  = 30 * time.Second
	supportCooldown     = 2 * time.Second
	supportHumanReply   = "Thanks — I'm connecting you with a human, we'll reply here shortly."
	supportShortenReply = "Please shorten your question."
)

type supportUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  *supportMessage `json:"message"`
}

type supportMessage struct {
	Chat supportChat  `json:"chat"`
	From *supportUser `json:"from"`
	Text string       `json:"text"`
}

type supportChat struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type supportUser struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type supportAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      []supportUpdate `json:"result"`
	Description string          `json:"description"`
}

// RunSupportBot polls the dedicated support bot and handles customer messages
// until ctx is cancelled. Telegram, Brain, and persistence errors are logged so
// a transient dependency failure does not stop the service.
func RunSupportBot(ctx context.Context, db *sql.DB) {
	token := strings.TrimSpace(os.Getenv("JARVIS_SUPPORT_BOT_TOKEN"))
	if token == "" {
		log.Print("support bot token not configured")
		return
	}

	client := &http.Client{Timeout: supportPollTimeout + 10*time.Second}
	brain := NewBrainFromEnv()
	lastProcessed := make(map[int64]time.Time)
	var offset int64

	log.Print("NOXIOAI support bot listening")
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		updates, err := getSupportUpdates(ctx, client, token, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("support getUpdates: %v", err)
			if !supportRetryDelay(ctx) {
				return
			}
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if update.Message == nil {
				continue
			}

			text := strings.TrimSpace(update.Message.Text)
			if text == "" {
				continue
			}

			chatID := update.Message.Chat.ID
			now := time.Now()
			if last, ok := lastProcessed[chatID]; ok && now.Sub(last) < supportCooldown {
				continue
			}
			lastProcessed[chatID] = now

			handleSupportMessage(ctx, client, token, brain, db, update.Message, text)
		}
	}
}

func getSupportUpdates(ctx context.Context, client *http.Client, token string, offset int64) ([]supportUpdate, error) {
	values := url.Values{
		"offset":          {strconv.FormatInt(offset, 10)},
		"timeout":         {strconv.Itoa(int(supportPollTimeout / time.Second))},
		"allowed_updates": {`["message"]`},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.telegram.org/bot"+token+"/getUpdates?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("telegram %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result supportAPIResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Telegram response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram getUpdates: %s", result.Description)
	}
	return result.Result, nil
}

func handleSupportMessage(ctx context.Context, client *http.Client, token string, brain *Brain, db *sql.DB, message *supportMessage, customerText string) {
	chatID := message.Chat.ID
	username, customerLabel := supportCustomerIdentity(message)
	escalated := shouldEscalate(customerText)
	reply := supportHumanReply
	ownerAlert := ""

	if utf8.RuneCountInString(customerText) > 2000 {
		escalated = false
		reply = supportShortenReply
	} else if !escalated {
		answer, err := brain.Chat([]Message{
			{Role: "system", Content: noxioSupportSystemPrompt},
			{Role: "user", Content: customerText},
		}, nil)
		if err != nil {
			log.Printf("support Brain for chat %d: %v", chatID, err)
			escalated = true
		} else {
			answer = strings.TrimSpace(answer)
			if brainNeedsEscalation(answer) {
				escalated = true
			} else {
				reply = limitTelegramText(answer)
			}
		}
	}

	if escalated {
		reply = supportHumanReply
		ownerAlert = fmt.Sprintf("🆘 NOXIOAI support escalation\nCustomer: %s\nChat ID: %d\nMessage: %s", customerLabel, chatID, customerText)
	}

	if err := sendSupportMessage(ctx, client, token, chatID, reply); err != nil {
		log.Printf("support reply to chat %d: %v", chatID, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO support_messages (chat_id, username, customer_msg, bot_reply, escalated)
		VALUES ($1, $2, $3, $4, $5)`, chatID, username, customerText, reply, escalated); err != nil {
		log.Printf("support persist chat %d: %v", chatID, err)
	}
	if ownerAlert != "" {
		if err := SendTelegram(ownerAlert); err != nil {
			log.Printf("support owner alert for chat %d: %v", chatID, err)
		}
	}
}

func sendSupportMessage(ctx context.Context, client *http.Client, token string, chatID int64, text string) error {
	values := url.Values{
		"chat_id": {strconv.FormatInt(chatID, 10)},
		"text":    {text},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+token+"/sendMessage", strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram sendMessage: %s", result.Description)
	}
	return nil
}

func supportCustomerIdentity(message *supportMessage) (string, string) {
	username := message.Chat.Username
	firstName := message.Chat.FirstName
	if message.From != nil {
		if message.From.Username != "" {
			username = message.From.Username
		}
		if message.From.FirstName != "" {
			firstName = message.From.FirstName
		}
	}
	if username != "" {
		return username, "@" + username
	}
	if firstName != "" {
		return firstName, firstName
	}
	return "", fmt.Sprintf("chat %d", message.Chat.ID)
}

func shouldEscalate(text string) bool {
	normalized := strings.ToLower(text)
	for _, phrase := range []string{"talk to someone", "real person"} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	for _, field := range strings.FieldsFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		switch field {
		case "human", "agent", "person", "operator", "نماینده", "انسان":
			return true
		}
	}
	return false
}

func brainNeedsEscalation(answer string) bool {
	if strings.TrimSpace(answer) == "" {
		return true
	}
	normalized := strings.ToLower(answer)
	normalized = strings.NewReplacer("’", "'", "‘", "'").Replace(normalized)
	for _, signal := range []string{
		"i don't know",
		"i do not know",
		"i'm not sure",
		"i am not sure",
		"can't answer",
		"cannot answer",
		"unable to answer",
		"can't help with that",
		"cannot help with that",
		"unable to help with that",
		"can't assist",
		"cannot assist",
		"unable to assist",
		"human will follow up",
		"custom quote",
		"نمی‌دانم",
		"نمی دانم",
		"نمی‌توانم پاسخ",
		"نمی توانم پاسخ",
		"اطمینان ندارم",
	} {
		if strings.Contains(normalized, signal) {
			return true
		}
	}
	return false
}

func limitTelegramText(text string) string {
	const maxRunes = 4000
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

func supportRetryDelay(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
