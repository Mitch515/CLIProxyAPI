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
// possible — a single dot prompt with max output 1.
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
			"model":             model,
			"max_output_tokens": 16,
			"input": []map[string]any{
				{"role": "user", "content": []map[string]any{
					{"type": "input_text", "text": "."},
				}},
			},
		})
		return body, sdktranslator.FromString("codex")
	default:
		// Gemini family (gemini, gemini-cli, vertex, aistudio, antigravity, qwen, iflow, kimi)
		body, _ := json.Marshal(map[string]any{
			"contents": []map[string]any{
				{"role": "user", "parts": []map[string]any{{"text": "."}}},
			},
			"generationConfig": map[string]any{
				"maxOutputTokens": 1,
			},
		})
		return body, sdktranslator.FromString("gemini")
	}
}
