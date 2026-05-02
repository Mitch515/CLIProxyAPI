package warmup

import "strings"

// DefaultModels maps lowercased provider keys to the cheapest model the
// scheduler will ping when no per-account override is set. Updated as
// providers retire / add cheaper SKUs.
var DefaultModels = map[string]string{
	"claude":      "claude-haiku-4-5",
	"anthropic":   "claude-haiku-4-5",
	"codex":       "gpt-5-nano",
	"openai":      "gpt-5-nano",
	"gemini":      "gemini-2.5-flash-lite",
	"gemini-cli":  "gemini-2.5-flash-lite",
	"vertex":      "gemini-2.5-flash-lite",
	"aistudio":    "gemini-2.5-flash-lite",
	"antigravity": "gemini-2.5-flash-lite",
	"qwen":        "qwen-turbo",
	"iflow":       "iflow-fast",
	"kimi":        "moonshot-v1-8k",
}

// PickModel returns the per-account override if set, otherwise the
// provider default, otherwise empty string.
func PickModel(provider, override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return override
	}
	return DefaultModels[strings.ToLower(strings.TrimSpace(provider))]
}
