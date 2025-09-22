package telegram

import (
	"coreheadlines/tools"
	"coreheadlines/typesPkg"
	"math/rand"
	"strconv"

	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const telegramMaxLen = 4096

const (
	maxAttempts  = 3
	baseBackoff  = 1 * time.Second
	maxBackoff   = 12 * time.Second
	jitterPctNum = 20 // +/- up to 20% jitter
)

type LinkPreviewOptions struct {
	IsDisabled       *bool  `json:"is_disabled,omitempty"`
	URL              string `json:"url,omitempty"`
	PreferSmallMedia *bool  `json:"prefer_small_media,omitempty"`
	PreferLargeMedia *bool  `json:"prefer_large_media,omitempty"`
	ShowAboveText    *bool  `json:"show_above_text,omitempty"`
}

type tgAPIError struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after,omitempty"`
	} `json:"parameters"`
}

func boolp(b bool) *bool { return &b }

func buildLinkPreviewOptionsJSON(p typesPkg.MainStruct) (string, error) {
	opt := LinkPreviewOptions{
		PreferLargeMedia: boolp(false),
		PreferSmallMedia: boolp(true),
		ShowAboveText:    boolp(false),
		IsDisabled:       boolp(false),
	}

	if link := strings.TrimSpace(p.Link); link != "" {
		opt.URL = link // ensure preview is for post.Link
	}

	b, err := json.Marshal(opt)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func SendMessages(posts []typesPkg.MainStruct, botToken, channelID string) error {
	if len(posts) == 0 {
		return nil
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	client := &http.Client{Timeout: 15 * time.Second}

	for i, p := range posts {
		text := buildTelegramHTML(p)
		text = ensureMaxLen(text, telegramMaxLen)

		replyMarkup, _ := buildInlineKeyboard(p, channelID)

		form := url.Values{}
		form.Set("chat_id", channelID)
		form.Set("text", text)
		form.Set("parse_mode", "HTML")
		if replyMarkup != "" {
			form.Set("reply_markup", replyMarkup)
		}
		if lpoJSON, err := buildLinkPreviewOptionsJSON(p); err == nil && lpoJSON != "" {
			form.Set("link_preview_options", lpoJSON)
		}

		if err := postWithRetry(client, endpoint, form, p.GUID); err != nil {
			return err
		}

		if i < len(posts)-1 {
			time.Sleep(1500 * time.Millisecond)
		}
	}
	return nil
}

func postWithRetry(client *http.Client, endpoint string, form url.Values, guid string) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := client.PostForm(endpoint, form)
		if err != nil {
			// network issue -> retryable
			lastErr = fmt.Errorf("sendMessage failed for GUID %q: %w", guid, err)
			if attempt < maxAttempts {
				time.Sleep(backoffDelay(attempt))
				continue
			}
			return lastErr
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		apiErr := tgAPIError{}
		_ = json.Unmarshal(body, &apiErr)

		// Respect explicit retry_after if present (FloodWait / rate limit)
		if (resp.StatusCode == http.StatusTooManyRequests || apiErr.ErrorCode == http.StatusTooManyRequests) && apiErr.Parameters.RetryAfter > 0 {
			if attempt < maxAttempts {
				time.Sleep(time.Duration(apiErr.Parameters.RetryAfter) * time.Second)
				continue
			}
			return fmt.Errorf("telegram rate limited (retry_after=%ds) for GUID %q: %s", apiErr.Parameters.RetryAfter, guid, string(body))
		}

		// Respect Retry-After header if provided
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs > 0 && attempt < maxAttempts {
				time.Sleep(time.Duration(secs) * time.Second)
				continue
			}
		}

		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			lastErr = fmt.Errorf("telegram API status %d: %s", resp.StatusCode, string(body))
			if attempt < maxAttempts {
				time.Sleep(backoffDelay(attempt))
				continue
			}
			return lastErr
		}

		return fmt.Errorf("telegram API status %d: %s", resp.StatusCode, string(body))
	}

	// Should not reach here
	return lastErr
}

func backoffDelay(attempt int) time.Duration {
	delay := min(baseBackoff<<(attempt-1), maxBackoff)

	jitter := int64(delay) * int64(jitterPctNum) / 100
	if jitter <= 0 {
		return delay
	}

	offset := rand.Int63n(2*jitter+1) - jitter
	return time.Duration(int64(delay) + offset)
}

func buildTelegramHTML(p typesPkg.MainStruct) string {
	rawTitle := strings.TrimSpace(p.Title)
	header := strings.TrimSpace(p.Header)

	emojis := strings.TrimSpace(tools.GetEmojis(rawTitle))

	var titleParts []string
	if emojis != "" {
		titleParts = append(titleParts, emojis)
	}
	if header != "" {
		h := header
		if !strings.HasSuffix(h, ":") {
			h += ":"
		}
		titleParts = append(titleParts, h)
	}
	if rawTitle != "" {
		titleParts = append(titleParts, rawTitle)
	}
	titleLine := strings.TrimSpace(strings.Join(titleParts, " "))

	var b strings.Builder

	if titleLine != "" {
		b.WriteString("<b>")
		b.WriteString(html.EscapeString(titleLine))
		b.WriteString("</b>")
	}

	return strings.TrimSpace(b.String())
}

func buildInlineKeyboard(p typesPkg.MainStruct, channelID string) (string, error) {
	link := strings.TrimSpace(p.Link)
	ch := strings.TrimSpace(channelID)

	var boostURL string
	if after, ok := strings.CutPrefix(ch, "@"); ok {
		boostURL = "https://t.me/" + after + "?boost"
	} else if strings.HasPrefix(ch, "-100") && len(ch) > 4 {
		boostURL = "https://t.me/c/" + ch[4:] + "?boost"
	} else if _, err := strconv.ParseInt(ch, 10, 64); err == nil {
		boostURL = "https://t.me/c/" + ch + "?boost"
	}

	type btn struct {
		Text         string `json:"text"`
		URL          string `json:"url,omitempty"`
		CallbackData string `json:"callback_data,omitempty"`
	}
	type markup struct {
		InlineKeyboard [][]btn `json:"inline_keyboard"`
	}

	row := make([]btn, 0, 2)
	if boostURL != "" {
		row = append(row, btn{Text: "⚡️ Boost", URL: boostURL})
	} else {
		// Fallback: still sends a callback to bot if deep link couldn't be built
		row = append(row, btn{Text: "⚡️ Boost", CallbackData: ensureMaxBytes("boost:"+p.GUID, 64)})
	}
	if link != "" {
		row = append(row, btn{Text: "🔗 Read", URL: link})
	}

	m := markup{InlineKeyboard: [][]btn{row}}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ensureMaxLen(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// Telegram limits callback_data to 1–64 bytes; trim if needed.
func ensureMaxBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// naive byte trim with ellipsis; safe since ASCII expected for GUID/prefix
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
