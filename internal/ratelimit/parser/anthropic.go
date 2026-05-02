package parser

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

// Anthropic exposes "unified" rate-limit headers covering combined input
// + output token usage per window. The names below are the documented set
// (lowercased here because http.Header.Get is case-insensitive but we also
// support legacy aliases that some proxies forward).
//
// Reference shape (May 2026, subject to drift):
//
//	anthropic-ratelimit-unified-5h-status: allowed | allowed_warning | rejected
//	anthropic-ratelimit-unified-5h-input-tokens: 12345
//	anthropic-ratelimit-unified-5h-output-tokens: 6789
//	anthropic-ratelimit-unified-5h-input-tokens-limit: 250000
//	anthropic-ratelimit-unified-5h-output-tokens-limit: 50000
//	anthropic-ratelimit-unified-5h-reset: 2026-05-02T18:30:00Z
//	(same set with -7d- prefix)
//
// Some forks/proxies rewrite the prefix. We tolerate both
// "anthropic-ratelimit-unified-5h-" and the legacy
// "anthropic-ratelimit-tokens-5h-" form.
const (
	anthroPrefixUnified = "anthropic-ratelimit-unified-"
	anthroPrefixLegacy  = "anthropic-ratelimit-tokens-"
)

func parseAnthropic(status int, headers http.Header, body []byte) []ratelimit.Observation {
	if headers == nil {
		return nil
	}
	var out []ratelimit.Observation
	for _, w := range []ratelimit.Window{ratelimit.Window5h, ratelimit.Window7d} {
		if obs, ok := readAnthropicWindow(headers, w); ok {
			if status == http.StatusTooManyRequests {
				obs.Exhausted = true
			}
			obs.Source = ratelimit.SourceHeader
			out = append(out, obs)
		}
	}
	// Some 429 responses omit headers entirely. In that case we cannot tell
	// which window was hit, but we can at least mark the account exhausted on
	// both windows so the warmup scheduler will retry both reset times.
	if status == http.StatusTooManyRequests && len(out) == 0 {
		now := time.Now().UTC()
		for _, w := range []ratelimit.Window{ratelimit.Window5h, ratelimit.Window7d} {
			out = append(out, ratelimit.Observation{
				Window:     w,
				PctUsed:    1.0,
				Exhausted:  true,
				ObservedAt: now,
				Source:     ratelimit.SourceErrorBody,
			})
		}
	}
	return out
}

func readAnthropicWindow(h http.Header, w ratelimit.Window) (ratelimit.Observation, bool) {
	suffix := string(w) + "-"
	prefixes := []string{anthroPrefixUnified + suffix, anthroPrefixLegacy + suffix}

	get := func(field string) string {
		for _, p := range prefixes {
			if v := strings.TrimSpace(h.Get(p + field)); v != "" {
				return v
			}
		}
		return ""
	}

	statusStr := strings.ToLower(get("status"))
	inputUsed := parseInt(get("input-tokens"))
	outputUsed := parseInt(get("output-tokens"))
	inputLimit := parseInt(get("input-tokens-limit"))
	outputLimit := parseInt(get("output-tokens-limit"))
	resetStr := get("reset")

	used := inputUsed + outputUsed
	limit := inputLimit + outputLimit
	resetAt, _ := parseTimestamp(resetStr)

	// Some headers report only "tokens" (combined) instead of input/output split.
	if used == 0 && limit == 0 && statusStr == "" && resetAt.IsZero() {
		combinedUsed := parseInt(get("tokens"))
		combinedLimit := parseInt(get("tokens-limit"))
		if combinedUsed == 0 && combinedLimit == 0 {
			return ratelimit.Observation{}, false
		}
		used = combinedUsed
		limit = combinedLimit
	}

	pct := 0.0
	if limit > 0 {
		pct = float64(used) / float64(limit)
		if pct > 1 {
			pct = 1
		}
	}

	exhausted := statusStr == "rejected" || pct >= 1.0
	return ratelimit.Observation{
		Window:      w,
		PctUsed:     pct,
		TokensUsed:  used,
		TokensLimit: limit,
		ResetAt:     resetAt,
		Exhausted:   exhausted,
	}, true
}

func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseTimestamp(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	s = strings.TrimSpace(s)
	// RFC3339 with or without sub-second precision.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true
	}
	// Unix seconds fallback (some providers report epoch).
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), true
	}
	return time.Time{}, false
}
