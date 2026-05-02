package parser

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

func TestAnthropicHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-status", "allowed_warning")
	h.Set("anthropic-ratelimit-unified-5h-input-tokens", "120000")
	h.Set("anthropic-ratelimit-unified-5h-output-tokens", "30000")
	h.Set("anthropic-ratelimit-unified-5h-input-tokens-limit", "200000")
	h.Set("anthropic-ratelimit-unified-5h-output-tokens-limit", "50000")
	h.Set("anthropic-ratelimit-unified-5h-reset", "2026-05-02T18:30:00Z")
	h.Set("anthropic-ratelimit-unified-7d-status", "rejected")
	h.Set("anthropic-ratelimit-unified-7d-input-tokens", "1000000")
	h.Set("anthropic-ratelimit-unified-7d-output-tokens", "200000")
	h.Set("anthropic-ratelimit-unified-7d-input-tokens-limit", "1000000")
	h.Set("anthropic-ratelimit-unified-7d-output-tokens-limit", "200000")
	h.Set("anthropic-ratelimit-unified-7d-reset", "2026-05-08T11:00:00Z")

	out := Parse("claude", 200, h, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(out))
	}

	got := map[ratelimit.Window]ratelimit.Observation{}
	for _, o := range out {
		got[o.Window] = o
	}
	if !approxEq(got[ratelimit.Window5h].PctUsed, 0.6, 0.0001) {
		t.Errorf("5h pct = %v, want ~0.6", got[ratelimit.Window5h].PctUsed)
	}
	if got[ratelimit.Window5h].Exhausted {
		t.Errorf("5h should not be exhausted at 60%%")
	}
	if !got[ratelimit.Window7d].Exhausted {
		t.Errorf("7d should be exhausted (status=rejected)")
	}
	if got[ratelimit.Window5h].ResetAt.IsZero() {
		t.Errorf("5h ResetAt should be parsed")
	}
}

func TestAnthropic429NoHeaders(t *testing.T) {
	out := Parse("claude", 429, http.Header{}, nil)
	if len(out) != 2 {
		t.Fatalf("expected fallback observations on bare 429, got %d", len(out))
	}
	for _, o := range out {
		if !o.Exhausted || o.PctUsed < 1.0 {
			t.Errorf("expected exhausted=true pct>=1 for window %s, got %+v", o.Window, o)
		}
	}
}

func TestOpenAIChatGPT429Body(t *testing.T) {
	body := []byte(`{"error":{"code":"usage_limit_exceeded","message":"You've hit the 5-hour usage cap. Please try again in 2h 14m."}}`)
	out := Parse("codex", 429, nil, body)
	if len(out) == 0 {
		t.Fatalf("expected at least one observation")
	}
	o := out[len(out)-1]
	if o.Window != ratelimit.Window5h {
		t.Errorf("window = %v, want 5h", o.Window)
	}
	if !o.Exhausted {
		t.Errorf("expected exhausted=true")
	}
	if o.ResetAt.IsZero() {
		t.Fatalf("expected ResetAt to be derived from message")
	}
	d := time.Until(o.ResetAt)
	if d < 2*time.Hour+10*time.Minute || d > 2*time.Hour+20*time.Minute {
		t.Errorf("ResetAt delta = %v, want ~2h14m", d)
	}
}

func TestOpenAIWeeklyMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"You hit your weekly cap. Try again in 3 days."}}`)
	out := Parse("codex", 429, nil, body)
	if len(out) == 0 {
		t.Fatalf("expected observation for weekly cap")
	}
	o := out[len(out)-1]
	if o.Window != ratelimit.Window7d {
		t.Errorf("window = %v, want 7d", o.Window)
	}
}

func TestGeminiQuotaFailure(t *testing.T) {
	body := []byte(`{
		"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[
			{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[
				{"quotaId":"GenerateContentRequestsPerDayPerProjectPerModel-FreeTier"}
			]},
			{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"60s"}
		]}
	}`)
	out := Parse("gemini", 429, nil, body)
	if len(out) == 0 {
		t.Fatalf("expected gemini observation")
	}
	o := out[0]
	if o.Window != ratelimit.Window7d {
		t.Errorf("PerDay quota should map to 7d, got %v", o.Window)
	}
	if d := time.Until(o.ResetAt); d < 50*time.Second || d > 70*time.Second {
		t.Errorf("retry delay = %v, want ~60s", d)
	}
}

func TestUnknownProvider(t *testing.T) {
	if got := Parse("nope", 429, nil, []byte(`{}`)); got != nil {
		t.Errorf("unknown provider should yield nil, got %v", got)
	}
}

func TestAnthropicLowercaseHeaderName(t *testing.T) {
	h := http.Header{}
	h.Add("Anthropic-Ratelimit-Unified-5h-Input-Tokens", "100")
	h.Add("Anthropic-Ratelimit-Unified-5h-Output-Tokens", "100")
	h.Add("Anthropic-Ratelimit-Unified-5h-Input-Tokens-Limit", "200")
	h.Add("Anthropic-Ratelimit-Unified-5h-Output-Tokens-Limit", "200")
	out := Parse("claude", 200, h, nil)
	if len(out) == 0 {
		t.Fatalf("expected case-insensitive header read")
	}
	if !strings.EqualFold(string(out[0].Window), "5h") {
		t.Errorf("window = %v", out[0].Window)
	}
}

func approxEq(a, b, tol float64) bool {
	if a-b > tol {
		return false
	}
	if b-a > tol {
		return false
	}
	return true
}
