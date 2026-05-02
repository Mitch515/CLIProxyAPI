package warmup

import "strings"

// DefaultModels maps lowercased provider keys to the cheapest model the
// scheduler will ping when no per-account override is set. Updated as
// providers retire / add cheaper SKUs.
var DefaultModels = map[string]string{
	"claude":    "claude-haiku-4-5",
	"anthropic": "claude-haiku-4-5",
	// Codex over ChatGPT subscriptions has a strict server-side allowlist:
	// gpt-5, gpt-5-codex, gpt-5.1, and the *-mini variants are all rejected
	// with "model not supported when using Codex with a ChatGPT account".
	// Empirical probing (scripts/probe-codex.ps1) shows that gpt-5.2 and
	// gpt-5.3-codex are accepted by ChatGPT-Pro accounts. We default to
	// gpt-5.2 since it's a non-codex SKU, less likely to count against the
	// premium codex weekly cap.
	"codex":  "gpt-5.2",
	"openai": "gpt-5.2",
	// Gemini (free tier) accepts gemini-2.5-flash but not -lite via OAuth.
	"gemini":      "gemini-2.5-flash",
	"gemini-cli":  "gemini-2.5-flash",
	"vertex":      "gemini-2.5-flash",
	"aistudio":    "gemini-2.5-flash",
	"antigravity": "gemini-2.5-flash",
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

// CandidateModels returns an ordered list of models the auto-detect endpoint
// will try for one provider. Cheap-and-likely-to-work first; we stop at the
// first one that succeeds. Empty for providers we don't know about — those
// fall back to the per-account override the user sets manually.
var candidateModels = map[string][]string{
	"codex": {
		// gpt-5.2 and gpt-5.3-codex are the two SKUs we've confirmed pass
		// the ChatGPT-Pro account allowlist (May 2026). Older codex models
		// (gpt-5, gpt-5-codex, gpt-5.1, gpt-5.1-codex-*) are server-side
		// rejected with "model not supported when using Codex with a
		// ChatGPT account".
		"gpt-5.2",
		"gpt-5.3-codex",
		"gpt-5.3-codex-spark",
		"gpt-5.2-codex",
		"gpt-5.1-codex-mini",
		"gpt-5-codex-mini",
		"gpt-5",
		"gpt-5.1",
		"gpt-5-codex",
	},
	"openai": {
		"gpt-5.2",
		"gpt-5.3-codex",
	},
	"claude": {
		"claude-haiku-4-5",
		"claude-sonnet-4-5",
	},
	"gemini": {
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-2.5-pro",
	},
	"gemini-cli": {
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
	},
	"vertex": {
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
	},
}

// CandidateModels returns the candidate list for a provider. Falls back to
// the single default model when no list is registered.
func CandidateModels(provider string) []string {
	key := strings.ToLower(strings.TrimSpace(provider))
	if list, ok := candidateModels[key]; ok {
		return list
	}
	if def, ok := DefaultModels[key]; ok {
		return []string{def}
	}
	return nil
}
