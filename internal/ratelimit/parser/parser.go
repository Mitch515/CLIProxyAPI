// Package parser converts upstream HTTP response metadata (headers and 429
// bodies) into a slice of rate-limit observations.
//
// Provider implementations are pure: they take headers + body and return a
// slice of observations. They never call the network, never touch the DB,
// and never log. This makes them trivially unit-testable against captured
// fixtures.
package parser

import (
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

// Parse dispatches by provider key (lowercase) and returns observations
// extracted from headers + body. The body may be nil for 2xx responses where
// only headers carry the data; conversely, headers may be empty for 429
// responses where the body is the only signal.
//
// AuthID, Email, and Label fields on returned observations are left blank;
// the publisher fills them in from the request context.
func Parse(provider string, status int, headers http.Header, body []byte) []ratelimit.Observation {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude", "anthropic":
		return parseAnthropic(status, headers, body)
	case "codex", "openai", "openai-compatibility":
		return parseOpenAI(status, headers, body)
	case "gemini", "gemini-cli", "vertex", "aistudio", "antigravity":
		return parseGemini(status, headers, body)
	default:
		return nil
	}
}
