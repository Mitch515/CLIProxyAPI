// Package ratelimit provides per-account rate-limit window tracking, persistence,
// and live broadcasting for the dashboard subsystem.
//
// This root package only declares the shared types and a global Bus used by
// the executor publishing helper to fan observations out to the service
// coordinator without import cycles. Subpackages (parser, store, service,
// warmup) import this package.
package ratelimit

import "time"

// Window identifies a rate-limit window kind reported by an upstream provider.
type Window string

const (
	// Window5h is the five-hour rolling window enforced by Anthropic Claude
	// (and the practical "5h block" enforced server-side by the ChatGPT-backed
	// Codex usage cap).
	Window5h Window = "5h"
	// Window7d is the seven-day rolling window enforced by Anthropic Claude.
	Window7d Window = "7d"
)

// Source describes how an observation was derived.
type Source string

const (
	// SourceHeader means we parsed structured rate-limit headers from a 2xx response.
	SourceHeader Source = "header"
	// SourceErrorBody means we parsed a 429 (or similar) error body.
	SourceErrorBody Source = "error_body"
)

// Observation is a single rate-limit datapoint extracted from one upstream
// response. Multiple observations may be produced from a single response (one
// per window).
type Observation struct {
	// AuthID is the runtime ID of the auth credential that owned the request.
	AuthID string
	// Provider is a lowercase provider key (claude, codex, gemini, ...).
	Provider string
	// Email is an optional account identifier captured opportunistically; it
	// makes account discovery easier in the dashboard.
	Email string
	// Label is an optional human-readable label for the auth.
	Label string
	// Window identifies which rolling window this datapoint describes.
	Window Window
	// PctUsed is the fraction of the window used in [0, 1]. A value >= 1
	// means the window is exhausted.
	PctUsed float64
	// TokensUsed is the total tokens consumed in the window when known.
	TokensUsed int64
	// TokensLimit is the limit for the window when known.
	TokensLimit int64
	// ResetAt is when the window will reset (zero if unknown).
	ResetAt time.Time
	// ObservedAt is when the observation was made (defaults to time.Now in
	// the publisher when zero).
	ObservedAt time.Time
	// Source records whether the data came from a response header or a 429 body.
	Source Source
	// Exhausted records whether the upstream explicitly reported the window
	// as exhausted (e.g. Anthropic status=rejected or HTTP 429). When the
	// derived PctUsed is < 1 but Exhausted is true, the service still marks
	// the account exhausted.
	Exhausted bool
}

// WindowState is the latest persisted state for a single window.
type WindowState struct {
	PctUsed     float64   `json:"pct_used"`
	TokensUsed  int64     `json:"tokens_used"`
	TokensLimit int64     `json:"tokens_limit,omitempty"`
	ResetAt     time.Time `json:"reset_at,omitempty"`
	Exhausted   bool      `json:"exhausted"`
	ObservedAt  time.Time `json:"observed_at,omitempty"`
}

// AccountSnapshot is the projection returned by the management API list/get endpoints.
type AccountSnapshot struct {
	ID            string                 `json:"id"`
	Provider      string                 `json:"provider"`
	Email         string                 `json:"email,omitempty"`
	Label         string                 `json:"label,omitempty"`
	Status        string                 `json:"status"`
	WarmupEnabled bool                   `json:"warmup_enabled"`
	WarmupModel   string                 `json:"warmup_model,omitempty"`
	Windows       map[Window]WindowState `json:"windows"`
	CreatedAt     time.Time              `json:"created_at,omitempty"`
	UpdatedAt     time.Time              `json:"updated_at,omitempty"`
	LastWarmup    *WarmupRecord          `json:"last_warmup,omitempty"`
}

// WarmupRecord describes a single warmup ping result.
type WarmupRecord struct {
	FiredAt time.Time `json:"fired_at"`
	Trigger string    `json:"trigger"`
	Model   string    `json:"model"`
	OK      bool      `json:"ok"`
	Error   string    `json:"error,omitempty"`
}
