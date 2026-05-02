package parser

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

// Gemini / Vertex / AIStudio parsing.
//
// Gemini APIs do not return rate-limit headers on success; instead 429
// responses include rich error details:
//
//	{ "error": {
//	    "code": 429,
//	    "status": "RESOURCE_EXHAUSTED",
//	    "details": [
//	      { "@type": ".../QuotaFailure",
//	        "violations": [{ "quotaId": "GenerateContentRequestsPerDayPerProjectPerModel-FreeTier", ... }]
//	      },
//	      { "@type": ".../RetryInfo", "retryDelay": "60s" }
//	    ]
//	} }
//
// We map quota IDs containing "Day" or "Daily" to the 7d window (best
// approximation; real "weekly" Gemini quotas are rare), and PerMinute /
// PerHour quotas to the 5h window. retryDelay is parsed as a duration.
func parseGemini(status int, headers http.Header, body []byte) []ratelimit.Observation {
	if status != http.StatusTooManyRequests || len(body) == 0 {
		return nil
	}
	var env geminiErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	if env.Error.Code != http.StatusTooManyRequests && !strings.EqualFold(env.Error.Status, "RESOURCE_EXHAUSTED") {
		return nil
	}

	var retryAt time.Time
	for _, d := range env.Error.Details {
		if strings.HasSuffix(d.Type, "RetryInfo") {
			if dur, err := time.ParseDuration(d.RetryDelay); err == nil {
				retryAt = time.Now().UTC().Add(dur)
			}
		}
	}

	windows := map[ratelimit.Window]bool{}
	for _, d := range env.Error.Details {
		if !strings.HasSuffix(d.Type, "QuotaFailure") {
			continue
		}
		for _, v := range d.Violations {
			id := strings.ToLower(v.QuotaID)
			switch {
			case strings.Contains(id, "perday"), strings.Contains(id, "daily"):
				windows[ratelimit.Window7d] = true
			case strings.Contains(id, "perminute"), strings.Contains(id, "perhour"), strings.Contains(id, "perhh"):
				windows[ratelimit.Window5h] = true
			default:
				// Unknown bucket — attribute to the 5h window which is the
				// more user-actionable (resets sooner).
				windows[ratelimit.Window5h] = true
			}
		}
	}

	if len(windows) == 0 {
		windows[ratelimit.Window5h] = true
	}

	out := make([]ratelimit.Observation, 0, len(windows))
	for w := range windows {
		out = append(out, ratelimit.Observation{
			Window:    w,
			PctUsed:   1.0,
			Exhausted: true,
			ResetAt:   retryAt,
			Source:    ratelimit.SourceErrorBody,
		})
	}
	return out
}

type geminiErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Details []struct {
			Type       string `json:"@type"`
			RetryDelay string `json:"retryDelay"`
			Violations []struct {
				QuotaID string `json:"quotaId"`
			} `json:"violations"`
		} `json:"details"`
	} `json:"error"`
}
