package parser

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

// OpenAI / Codex parsing.
//
// The /v1/responses endpoint and most OpenAI-compatible upstreams set the
// canonical x-ratelimit-* headers on 2xx responses:
//
//	x-ratelimit-limit-tokens, x-ratelimit-remaining-tokens, x-ratelimit-reset-tokens
//	x-ratelimit-limit-requests, x-ratelimit-remaining-requests, x-ratelimit-reset-requests
//
// The reset values are typically duration strings like "5m20s" or "2h30s". We
// parse them as time.ParseDuration with a regex fallback.
//
// For ChatGPT-backed Codex (the 5h cap that's enforced server-side rather
// than via the OpenAI-Pay API), the only reliable signal is the 429 body:
//
//	{ "error": { "code": "usage_limit_exceeded",
//	             "message": "You've hit the 5-hour usage cap. Please try again in 2h 14m." } }
//
// We extract any "X hours Y minutes" / "X minutes Y seconds" duration phrase
// and mark the 5h window exhausted with reset_at = now + that duration.
func parseOpenAI(status int, headers http.Header, body []byte) []ratelimit.Observation {
	// Prefer the Codex-specific x-codex-primary-*/x-codex-secondary-* headers
	// when they are present (ChatGPT-backed Codex via chatgpt.com). They
	// describe the real 5h/7d windows and supersede any x-ratelimit-*
	// values the upstream may also set.
	if codexObs := parseCodexHeaders(headers); len(codexObs) > 0 {
		if status == http.StatusTooManyRequests {
			for i := range codexObs {
				codexObs[i].Exhausted = true
			}
		}
		return codexObs
	}

	out := readOpenAIHeaders(headers)
	if status == http.StatusTooManyRequests {
		if obs, ok := parseOpenAI429Body(body); ok {
			out = append(out, obs)
		} else if len(out) == 0 {
			// Last-resort: mark 5h exhausted with no reset hint.
			out = append(out, ratelimit.Observation{
				Window:    ratelimit.Window5h,
				PctUsed:   1.0,
				Exhausted: true,
				Source:    ratelimit.SourceErrorBody,
			})
		}
		for i := range out {
			out[i].Exhausted = out[i].Exhausted || out[i].PctUsed >= 1.0
		}
	}
	return out
}

func readOpenAIHeaders(h http.Header) []ratelimit.Observation {
	if h == nil {
		return nil
	}
	limitTok := parseInt(h.Get("x-ratelimit-limit-tokens"))
	remTok := parseInt(h.Get("x-ratelimit-remaining-tokens"))
	resetTok := h.Get("x-ratelimit-reset-tokens")

	if limitTok == 0 && resetTok == "" {
		return nil
	}
	used := limitTok - remTok
	if used < 0 {
		used = 0
	}
	pct := 0.0
	if limitTok > 0 {
		pct = float64(used) / float64(limitTok)
		if pct > 1 {
			pct = 1
		}
	}
	resetAt, _ := parseResetClock(resetTok)
	return []ratelimit.Observation{{
		// Header-level x-ratelimit-* values typically describe the per-minute
		// or per-hour rolling bucket. We attribute them to the 5h window as
		// a best-effort; the 429 body parser supersedes them when both fire.
		Window:      ratelimit.Window5h,
		PctUsed:     pct,
		TokensUsed:  used,
		TokensLimit: limitTok,
		ResetAt:     resetAt,
		Source:      ratelimit.SourceHeader,
	}}
}

// parseResetClock accepts either a Go duration ("2h30m"), a bare integer
// (seconds-from-now), or an RFC3339 timestamp.
func parseResetClock(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(d), true
	}
	if t, ok := parseTimestamp(s); ok {
		return t, true
	}
	if n := parseInt(s); n > 0 {
		return time.Now().UTC().Add(time.Duration(n) * time.Second), true
	}
	return time.Time{}, false
}

// chatgpt usage-cap body shapes vary slightly; pull any duration phrase out
// of the message field.
type openaiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

var (
	rePhraseHM   = regexp.MustCompile(`(?i)(\d+)\s*h(?:ours?|rs?)?\s*(?:and\s*)?(\d+)?\s*m?`)
	reSinglePart = regexp.MustCompile(`(?i)(\d+)\s*(seconds?|secs?|minutes?|mins?|hours?|hrs?|days?)`)
)

func parseOpenAI429Body(body []byte) (ratelimit.Observation, bool) {
	if len(body) == 0 {
		return ratelimit.Observation{}, false
	}
	var env openaiErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return ratelimit.Observation{}, false
	}
	msg := env.Error.Message
	if msg == "" {
		return ratelimit.Observation{}, false
	}

	dur, ok := extractDuration(msg)
	if !ok {
		// Even without a duration, knowing this was a usage cap is useful.
		return ratelimit.Observation{
			Window:    pickOpenAIWindow(msg),
			PctUsed:   1.0,
			Exhausted: true,
			Source:    ratelimit.SourceErrorBody,
		}, true
	}
	return ratelimit.Observation{
		Window:    pickOpenAIWindow(msg),
		PctUsed:   1.0,
		Exhausted: true,
		ResetAt:   time.Now().UTC().Add(dur),
		Source:    ratelimit.SourceErrorBody,
	}, true
}

func pickOpenAIWindow(msg string) ratelimit.Window {
	low := strings.ToLower(msg)
	if strings.Contains(low, "weekly") || strings.Contains(low, "7-day") || strings.Contains(low, "7 day") || strings.Contains(low, "week") {
		return ratelimit.Window7d
	}
	return ratelimit.Window5h
}

// extractDuration tries hard to find a "wait this long" hint in a free-form
// message. It returns the parsed duration on success.
func extractDuration(msg string) (time.Duration, bool) {
	if m := rePhraseHM.FindStringSubmatch(msg); len(m) >= 2 {
		hours := parseInt(m[1])
		mins := int64(0)
		if len(m) >= 3 {
			mins = parseInt(m[2])
		}
		if hours > 0 || mins > 0 {
			return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute, true
		}
	}
	if m := reSinglePart.FindStringSubmatch(msg); len(m) >= 3 {
		n := parseInt(m[1])
		unit := strings.ToLower(m[2])
		switch {
		case strings.HasPrefix(unit, "sec"):
			return time.Duration(n) * time.Second, true
		case strings.HasPrefix(unit, "min"):
			return time.Duration(n) * time.Minute, true
		case strings.HasPrefix(unit, "hour"), strings.HasPrefix(unit, "hr"):
			return time.Duration(n) * time.Hour, true
		case strings.HasPrefix(unit, "day"):
			return time.Duration(n) * 24 * time.Hour, true
		}
	}
	return 0, false
}
