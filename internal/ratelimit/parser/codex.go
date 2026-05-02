package parser

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

// ChatGPT-backed Codex (chatgpt.com/backend-api/codex/responses) returns its
// own rate-limit headers on every successful streaming response. They differ
// from the standard OpenAI x-ratelimit-* family.
//
// Confirmed by reading the official Codex CLI source code
// (codex-rs/codex-api/src/rate_limits.rs / parse_rate_limit_for_limit):
//
//	x-codex-primary-used-percent      (float, 0-100)
//	x-codex-primary-window-minutes    (int — 300 for 5h)
//	x-codex-primary-reset-at          (int unix-seconds)
//	x-codex-secondary-used-percent    (float, 0-100)
//	x-codex-secondary-window-minutes  (int — 10080 for 7d)
//	x-codex-secondary-reset-at        (int unix-seconds)
//	x-codex-limit-name                (string, optional label)
//
// We map "primary" → 5h window and "secondary" → 7d window based on
// window-minutes, falling back to position if minutes are missing.
func parseCodexHeaders(h http.Header) []ratelimit.Observation {
	if h == nil {
		return nil
	}
	var out []ratelimit.Observation
	for _, kind := range []string{"primary", "secondary"} {
		if obs, ok := readCodexBucket(h, kind); ok {
			out = append(out, obs)
		}
	}
	return out
}

func readCodexBucket(h http.Header, kind string) (ratelimit.Observation, bool) {
	prefix := "x-codex-" + kind + "-"
	pctRaw := strings.TrimSpace(h.Get(prefix + "used-percent"))
	winRaw := strings.TrimSpace(h.Get(prefix + "window-minutes"))
	resetRaw := strings.TrimSpace(h.Get(prefix + "reset-at"))

	if pctRaw == "" && winRaw == "" && resetRaw == "" {
		return ratelimit.Observation{}, false
	}

	pct, _ := strconv.ParseFloat(pctRaw, 64)
	if pct > 1 && pct <= 100 {
		pct = pct / 100.0
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	winMin, _ := strconv.ParseInt(winRaw, 10, 64)
	window := pickCodexWindow(kind, winMin)

	var resetAt time.Time
	if reset, err := strconv.ParseInt(resetRaw, 10, 64); err == nil && reset > 0 {
		resetAt = time.Unix(reset, 0).UTC()
	}

	return ratelimit.Observation{
		Window:    window,
		PctUsed:   pct,
		ResetAt:   resetAt,
		Source:    ratelimit.SourceHeader,
		Exhausted: pct >= 1.0,
	}, true
}

// pickCodexWindow returns the canonical window key for a given primary/secondary
// bucket. We honor explicit window-minutes when present so that a future
// schedule shift (e.g. 4-hour primary) is recorded with the right label.
func pickCodexWindow(kind string, windowMinutes int64) ratelimit.Window {
	switch {
	case windowMinutes >= 6*24*60: // anything >= 6 days is the weekly bucket
		return ratelimit.Window7d
	case windowMinutes > 0 && windowMinutes < 6*24*60:
		return ratelimit.Window5h
	}
	if kind == "secondary" {
		return ratelimit.Window7d
	}
	return ratelimit.Window5h
}
