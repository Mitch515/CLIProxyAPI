package warmup

import (
	"encoding/json"
	"strings"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

// buildPayload returns a tiny one-token request body for the given provider
// in its native format. The Format value tells the executor which schema we
// are sending so it does not try to translate the payload again.
//
// We keep these payloads minimal so the warmup ping costs as little as
// possible — a single dot prompt with max output 1 — and we omit fields
// that some upstreams reject:
//   - Codex/ChatGPT: no max_output_tokens (upstream returns "Unsupported
//     parameter: max_output_tokens")
//   - Gemini CLI / Cloud Code Assist: payload is wrapped in {request:{...}}
//     because the executor sets project and model at the top level
func buildPayload(provider, model string) ([]byte, sdktranslator.Format) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "claude", "anthropic":
		body, _ := json.Marshal(map[string]any{
			"model":      model,
			"max_tokens": 1,
			"messages": []map[string]any{
				{"role": "user", "content": "."},
			},
		})
		return body, sdktranslator.FromString("claude")
	case "codex", "openai", "openai-compatibility":
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"input": []map[string]any{
				{"role": "user", "content": []map[string]any{
					{"type": "input_text", "text": "."},
				}},
			},
		})
		return body, sdktranslator.FromString("codex")
	case "gemini-cli", "antigravity", "vertex", "aistudio":
		// Cloud Code Assist envelope. The executor will fill in project
		// and model at the top level after this payload is built.
		body, _ := json.Marshal(map[string]any{
			"request": map[string]any{
				"contents": []map[string]any{
					{"role": "user", "parts": []map[string]any{{"text": "."}}},
				},
				"generationConfig": map[string]any{"maxOutputTokens": 1},
			},
		})
		return body, sdktranslator.FromString("gemini-cli")
	default:
		// Generic Gemini API (and fallback for everything else).
		body, _ := json.Marshal(map[string]any{
			"contents": []map[string]any{
				{"role": "user", "parts": []map[string]any{{"text": "."}}},
			},
			"generationConfig": map[string]any{"maxOutputTokens": 1},
		})
		return body, sdktranslator.FromString("gemini")
	}
}
