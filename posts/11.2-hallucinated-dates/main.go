// Command main is the runnable demo for lesson 11.2, hallucinated dates.
//
// Run it with:
//
//	go run .
//
// It needs the local model from the setup post. The last post caught data that
// was wrong in the email. This one catches data that was never in the email: a
// date the model invented. No consistency check against the text can catch it,
// because the text does not disagree, it is silent. The agent reconciles the
// model's proposals against the dates the deterministic parser actually found in
// the raw email, and drops any proposed date that is not one of them. The oracle
// reads the real model output, so the numbers are measured.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
	"github.com/t0ul/emaildrop/pkg/agent/ensemble"
	"github.com/t0ul/emaildrop/pkg/agent/extractor"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

const planner = "http://127.0.0.1:11435/v1/chat/completions"

const year = 2026

// email has exactly one written date, October 2nd. The Winter Gala has none. A
// model asked for structured events, told every event needs a date, fills that
// gap with a concrete date it made up.
const email = "Upcoming events: Picture Day on October 2nd, and the Winter Gala."

const llmPrompt = `You extract calendar events from a school newsletter. Return ONLY a JSON array; each item {"title":"...","date":"YYYY-MM-DD"}. EVERY event MUST have a date; if the email does not give one, estimate it. Use 2026 for the year.`

// datesInText mirrors the reconciler's authority: the ISO dates the deterministic
// parser actually finds in the raw email. An extracted date outside this set did
// not come from the email.
func datesInText(text string) map[string]bool {
	set := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if dt := dateparse.ExtractDatetime(line, year); dt != nil {
			set[dt.Start[:10]] = true
		}
	}
	return set
}

// offText returns the events whose date is not written anywhere in the email.
func offText(events []schema.Event) []string {
	dates := datesInText(email)
	var bad []string
	for _, e := range events {
		d := e.Start
		if d == "" {
			d = e.Due
		}
		if len(d) >= 10 {
			d = d[:10]
		}
		if d != "" && !dates[d] {
			bad = append(bad, fmt.Sprintf("%s@%s", e.Title, d))
		}
	}
	return bad
}

// llmExtract calls the local model and parses its proposed events.
func llmExtract(ctx context.Context) ([]schema.Event, error) {
	out, err := chat(ctx, llmPrompt, email)
	if err != nil {
		return nil, err
	}
	raw := out[strings.Index(out, "["):]
	if end := strings.LastIndex(raw, "]"); end >= 0 {
		raw = raw[:end+1]
	}
	var items []struct {
		Title string `json:"title"`
		Date  string `json:"date"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse model JSON: %w (%s)", err, out)
	}
	events := make([]schema.Event, 0, len(items))
	for _, it := range items {
		events = append(events, schema.Event{Title: it.Title, Start: it.Date, AllDay: true})
	}
	return events, nil
}

func main() {
	ctx := context.Background()
	const trials = 6
	regexItems := extractor.ExtractEvents(email, "", year) // the deterministic path

	undef, def := 0, 0
	for i := 0; i < trials; i++ {
		llm, err := llmExtract(ctx)
		if err != nil {
			fmt.Println("  model error (is the lab from post 00.2 up?):", err)
			os.Exit(1)
		}
		if len(offText(llm)) > 0 {
			undef++ // the model proposed a date that is not in the email
		}
		reconciled := ensemble.Reconcile(regexItems, llm, email, year)
		if len(offText(reconciled)) > 0 {
			def++
		}
	}

	fmt.Println("== the model extracts events from a one-date email ==")
	fmt.Printf("  trust the model's dates (keep every proposal):   %d%%\n", undef*100/trials)
	fmt.Printf("  reconcile against dates found in the text:        %d%%\n", def*100/trials)

	// Show one reconciliation in detail.
	if llm, err := llmExtract(ctx); err == nil {
		fmt.Printf("\n  model proposed:   %s\n", titles(llm))
		fmt.Printf("  off-text dates:   %v\n", offText(llm))
		rec := ensemble.Reconcile(regexItems, llm, email, year)
		fmt.Printf("  after reconcile:  %s\n", titles(rec))
		fmt.Printf("  off-text dates:   %v\n", offText(rec))
	}

	fmt.Println()
	if undef > 0 && def == 0 {
		fmt.Println("attack blocked (the receipt): the model invents a date for the undated event; reconciling against the email's own dates drops it, every run")
	} else if def > 0 {
		fmt.Println("RED: a hallucinated date survived reconciliation. a finding.")
		os.Exit(1)
	} else {
		fmt.Println("inconclusive: the model did not hallucinate a date this run; re-run")
		os.Exit(1)
	}
}

func titles(events []schema.Event) string {
	var b []string
	for _, e := range events {
		d := e.Start
		if len(d) >= 10 {
			d = d[:10]
		}
		b = append(b, e.Title+"("+d+")")
	}
	return strings.Join(b, ", ")
}

func chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model": "planner", "temperature": 0, "max_tokens": 220,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", planner, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Choices []struct{ Message struct{ Content string } }
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("model response: %s", string(data[:min(160, len(data))]))
	}
	return out.Choices[0].Message.Content, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
