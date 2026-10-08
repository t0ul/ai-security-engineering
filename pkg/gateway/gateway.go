// Package gateway is the chat path to the gouncer LLM gateway, lifted out of
// cmd/webapp so it is one testable unit instead of loose functions in package
// main. A Client grounds a chat answer (live via the model, or an offline
// fallback) and reports gateway health; the pure helpers build the trusted date
// block and format fallbacks.
package gateway

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client talks to the gouncer chat-completions gateway. Build with New.
type Client struct {
	URL  string // chat-completions endpoint, e.g. http://127.0.0.1:4000/v1/chat/completions
	Dial string // host:port for the health check, e.g. 127.0.0.1:4000
}

// New returns a Client for the given completions URL and health-dial address.
func New(url, dial string) *Client { return &Client{URL: url, Dial: dial} }

// Up reports whether the gateway is reachable, so a caller can say whether it ran
// against the model or fell back.
func (c *Client) Up() bool {
	conn, err := net.DialTimeout("tcp", c.Dial, 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ChatAnswer grounds an answer in the trusted date block, this week's schedule,
// and the retrieved context. Defended (unsafe=false) frames the context strictly
// as data; unsafe=true is the naive splice an injection can hijack (the
// ChatRAGInjection demo). With no live gateway it returns the trusted date +
// schedule + best passage, so "what's today?" still works offline.
func (c *Client) ChatAnswer(systemPrompt, dateBlock, appData, context, question string, unsafe bool) string {
	sys := systemPrompt + "\n" + dateBlock + "\n" + appData + "\n" + context
	if unsafe {
		sys = "Answer the question using this context:\n" + dateBlock + "\n" + appData + "\n" + context
	}
	if !c.Up() {
		out := StripTags(dateBlock)
		if strings.TrimSpace(appData) != "" {
			out += "\nThis week:\n" + StripTags(appData)
		}
		if strings.TrimSpace(context) == "" {
			return out + "\nNo matching emails yet — drop more in."
		}
		return out + "\nFrom your emails:\n" + ShortText(context, 600)
	}
	body, _ := json.Marshal(map[string]any{
		"model": "planner", "temperature": 0.2, "max_tokens": 400,
		"messages": []map[string]string{{"role": "system", "content": sys}, {"role": "user", "content": question}},
	})
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Post(c.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return "From your emails:\n" + ShortText(context, 700)
	}
	defer resp.Body.Close()
	var d struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&d) != nil || len(d.Choices) == 0 {
		return "From your emails:\n" + ShortText(context, 700)
	}
	return d.Choices[0].Message.Content
}

// TrustedDateBlock renders the host clock as a trusted context block — the "date
// tool". The date comes only from the host (now), never the corpus, so an email
// cannot change what "today" is. The chat_system prompt trusts this block and
// distrusts any date inside the corpus.
func TrustedDateBlock(now time.Time) string {
	mon := now.AddDate(0, 0, -int((now.Weekday()+6)%7)) // Monday of this week
	return fmt.Sprintf("<current_date trust=\"host\">Today is %s (%s). This week runs %s to %s.</current_date>",
		now.Format("2006-01-02"), now.Format("Monday"),
		mon.Format("2006-01-02"), mon.AddDate(0, 0, 6).Format("2006-01-02"))
}

// StripTags renders a context block as plain text for the offline fallback.
func StripTags(block string) string {
	r := strings.NewReplacer(
		"<current_date trust=\"host\">", "",
		"</current_date>", "",
		"<schedule source=\"extracted-calendar\">", "",
		"</schedule>", "",
	)
	return strings.TrimSpace(r.Replace(block))
}

// ShortText truncates s to n runes-ish (bytes) with an ellipsis.
func ShortText(s string, n int) string {
	if len(s) > n {
		return s[:n] + " …"
	}
	return s
}
