package management

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/service"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/store"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/warmup"
)

// RateLimitDeps bundles the optional rate-limit subsystem references the
// management handler needs. They may all be nil when the dashboard
// subsystem is disabled — endpoints will return 503 in that case.
type RateLimitDeps struct {
	Store     *store.Store
	Service   *service.Service
	Scheduler *warmup.Scheduler
}

// SetRateLimitDeps wires the dashboard subsystem references.
func (h *Handler) SetRateLimitDeps(deps RateLimitDeps) {
	h.rl = deps
}

// SSEEnabled reports whether the SSE endpoint can serve clients.
func (h *Handler) SSEEnabled() bool {
	return h.rl.Service != nil
}

// ============================================================================
// Accounts list / get / patch
// ============================================================================

// ListAccounts returns the snapshot for every known account.
//
//	GET /v0/management/accounts
func (h *Handler) ListAccounts(c *gin.Context) {
	if h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	accounts, err := h.rl.Store.ListAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]ratelimit.AccountSnapshot, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, h.snapshotFor(c, a))
	}
	c.JSON(http.StatusOK, gin.H{"accounts": out})
}

// GetAccount returns one account snapshot.
//
//	GET /v0/management/accounts/:id
func (h *Handler) GetAccount(c *gin.Context) {
	if h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	id := c.Param("id")
	a, err := h.rl.Store.GetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, h.snapshotFor(c, a))
}

// PatchAccount mutates the user-controlled fields.
//
//	PATCH /v0/management/accounts/:id
func (h *Handler) PatchAccount(c *gin.Context) {
	if h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	id := c.Param("id")
	var body struct {
		Label         *string `json:"label"`
		WarmupEnabled *bool   `json:"warmup_enabled"`
		WarmupModel   *string `json:"warmup_model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	patch := store.AccountPatch{
		Label:         body.Label,
		WarmupEnabled: body.WarmupEnabled,
		WarmupModel:   body.WarmupModel,
	}
	updated, err := h.rl.Store.Patch(c.Request.Context(), id, patch)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, h.snapshotFor(c, updated))
}

// GetAccountHistory returns the time series for one window.
//
//	GET /v0/management/accounts/:id/history?window=5h&since=...&limit=500
func (h *Handler) GetAccountHistory(c *gin.Context) {
	if h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	id := c.Param("id")
	window := ratelimit.Window(strings.ToLower(c.DefaultQuery("window", "5h")))
	if window != ratelimit.Window5h && window != ratelimit.Window7d {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be 5h or 7d"})
		return
	}

	defaultLookback := 7 * 24 * time.Hour
	if window == ratelimit.Window7d {
		defaultLookback = 30 * 24 * time.Hour
	}
	since := time.Now().UTC().Add(-defaultLookback)
	if raw := c.Query("since"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			since = t.UTC()
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))

	points, err := h.rl.Store.History(c.Request.Context(), id, window, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type point struct {
		T       time.Time `json:"t"`
		Pct     float64   `json:"pct"`
		Tokens  int64     `json:"tokens,omitempty"`
		ResetAt time.Time `json:"reset_at,omitempty"`
		Source  string    `json:"source"`
	}
	pts := make([]point, len(points))
	for i, p := range points {
		pts[i] = point{T: p.ObservedAt, Pct: p.PctUsed, Tokens: p.TokensUsed, ResetAt: p.ResetAt, Source: string(p.Source)}
	}
	c.JSON(http.StatusOK, gin.H{"window": window, "points": pts})
}

// ============================================================================
// Manual warmup
// ============================================================================

// PostAccountWarmup fires a manual warmup ping for one account.
//
//	POST /v0/management/accounts/:id/warmup
func (h *Handler) PostAccountWarmup(c *gin.Context) {
	if h.rl.Scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "warmup subsystem disabled"})
		return
	}
	id := c.Param("id")
	if err := h.rl.Scheduler.FireOnce(c.Request.Context(), id, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec, _ := h.rl.Store.LastWarmup(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"fired": true, "last": rec})
}

// PostWarmupAll fires a warmup against every known account (any provider). The
// response is the per-account result of the run. Useful as a "ping every
// subscription" smoke button on the dashboard. Runs sequentially with a
// 30-second per-call timeout the scheduler already applies.
//
//	POST /v0/management/accounts/warmup-all
func (h *Handler) PostWarmupAll(c *gin.Context) {
	if h.rl.Scheduler == nil || h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "warmup subsystem disabled"})
		return
	}
	accs, err := h.rl.Store.ListAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type result struct {
		ID    string                  `json:"id"`
		OK    bool                    `json:"ok"`
		Last  *ratelimit.WarmupRecord `json:"last,omitempty"`
		Error string                  `json:"error,omitempty"`
	}
	out := make([]result, 0, len(accs))
	for _, a := range accs {
		if err := h.rl.Scheduler.FireOnce(c.Request.Context(), a.AuthID, true); err != nil {
			out = append(out, result{ID: a.AuthID, OK: false, Error: err.Error()})
			continue
		}
		rec, _ := h.rl.Store.LastWarmup(c.Request.Context(), a.AuthID)
		ok := rec != nil && rec.OK
		out = append(out, result{ID: a.AuthID, OK: ok, Last: rec})
	}
	c.JSON(http.StatusOK, gin.H{"results": out})
}

// PostAccountAutoDetect probes a list of candidate models against one
// account, sets warmup_model to the first model that succeeds, and returns
// the per-attempt log. The detection result is not persisted in
// rate_observations; only the chosen model is saved on the account.
//
//	POST /v0/management/accounts/:id/auto-detect
func (h *Handler) PostAccountAutoDetect(c *gin.Context) {
	if h.rl.Scheduler == nil || h.rl.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "warmup subsystem disabled"})
		return
	}
	id := c.Param("id")
	acc, err := h.rl.Store.GetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	candidates := warmup.CandidateModels(acc.Provider)
	if len(candidates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no candidate models for provider " + acc.Provider})
		return
	}

	type attempt struct {
		Model string `json:"model"`
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	attempts := make([]attempt, 0, len(candidates))
	winner := ""
	tokenExpired := false

	for _, model := range candidates {
		// Set this model as the warmup_model and fire one ping.
		_, _ = h.rl.Store.Patch(c.Request.Context(), id, store.AccountPatch{WarmupModel: &model})
		_ = h.rl.Scheduler.FireOnce(c.Request.Context(), id, true)
		rec, _ := h.rl.Store.LastWarmup(c.Request.Context(), id)
		if rec == nil {
			attempts = append(attempts, attempt{Model: model, OK: false, Error: "no warmup record"})
			continue
		}
		attempts = append(attempts, attempt{Model: model, OK: rec.OK, Error: rec.Error})
		if rec.OK {
			winner = model
			break
		}
		if isTokenExpiredError(rec.Error) {
			tokenExpired = true
			break
		}
	}

	// If nothing worked, restore the original warmup_model.
	if winner == "" {
		original := acc.WarmupModel
		_, _ = h.rl.Store.Patch(c.Request.Context(), id, store.AccountPatch{WarmupModel: &original})
	}

	c.JSON(http.StatusOK, gin.H{
		"winner":        winner,
		"token_expired": tokenExpired,
		"attempts":      attempts,
	})
}

func isTokenExpiredError(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "token_expired") ||
		strings.Contains(low, "authentication token is expired") ||
		strings.Contains(low, "refresh token is expired")
}

// DeleteAccount hides the account from the dashboard. If a real auth file
// backs the account it is also removed from disk; virtual sub-accounts (e.g.
// gemini-cli per-project entries that the watcher synthesizes on every start)
// just get hidden=true so they stop coming back. The user can re-show
// hidden accounts via the (TODO) "show hidden" toggle.
//
//	DELETE /v0/management/accounts/:id
func (h *Handler) DeleteAccount(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	id := c.Param("id")
	if id == "" || strings.Contains(id, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Real-file account IDs match a single filename under auth-dir. Virtual
	// IDs contain "::" (e.g. "<file>.json::<project-id>"). Slashes/backslashes
	// would also be path traversal — reject them for filename actions but
	// still allow virtual IDs to be hidden.
	provider := ""
	authDir := expandHome(h.cfg.AuthDir)
	isFilename := !strings.Contains(id, "::") && !strings.ContainsAny(id, `/\\`)
	if isFilename {
		path := filepath.Join(authDir, id)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if h.rl.Store != nil {
		// Look up provider before we hide the row so EnsureHiddenRow has a
		// useful value when no row exists yet.
		if existing, err := h.rl.Store.GetAccount(c.Request.Context(), id); err == nil {
			provider = existing.Provider
		}
		if err := h.rl.Store.EnsureHiddenRow(c.Request.Context(), id, provider); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Run reconcile after the file watcher has had a moment to react.
	if h.rl.Service != nil {
		go func() {
			time.Sleep(250 * time.Millisecond)
			h.rl.Service.ReconcileNow(context.Background())
		}()
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true, "hidden": true, "removed_file": isFilename})
}

// expandHome resolves a leading ~ in an auth-dir path.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

// ============================================================================
// SSE ticket + stream
// ============================================================================

// PostSSETicket mints a short-lived (60s) ticket. The dashboard exchanges its
// bearer token for a ticket, then opens an EventSource with ?ticket=… so the
// management auth never appears in URL access logs.
//
//	POST /v0/management/sse-ticket
func (h *Handler) PostSSETicket(c *gin.Context) {
	if !h.SSEEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	t, expiresAt, err := h.mintTicket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": t, "expires_at": expiresAt})
}

// RateLimitEventsSSE streams account.changed and warmup.fired events.
// Authentication uses the ticket query param, NOT the bearer middleware
// (the route is registered outside /v0/management to bypass it).
//
//	GET /v0/management/events?ticket=…
func (h *Handler) RateLimitEventsSSE(c *gin.Context) {
	if h.rl.Service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ratelimit subsystem disabled"})
		return
	}
	if !h.consumeTicket(c.Query("ticket")) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired ticket"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	events, cancel := h.rl.Service.Subscribe()
	defer cancel()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-heartbeat.C:
			if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case e, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(e.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\n", e.Kind)
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			c.Writer.Flush()
		}
	}
}

// snapshotFor builds the API view of a stored Account, layering in the
// most recent warmup record if available.
func (h *Handler) snapshotFor(c *gin.Context, a store.Account) ratelimit.AccountSnapshot {
	snap := ratelimit.AccountSnapshot{
		ID:            a.AuthID,
		Provider:      a.Provider,
		Email:         a.Email,
		Label:         a.Label,
		Status:        "active",
		WarmupEnabled: a.WarmupEnabled,
		WarmupModel:   a.WarmupModel,
		Windows:       map[ratelimit.Window]ratelimit.WindowState{},
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}

	if h.authManager != nil {
		if existing, ok := h.authManager.GetByID(a.AuthID); ok && existing != nil {
			snap.Windows[ratelimit.Window5h] = ratelimit.WindowState{
				PctUsed:    existing.Quota.Window5hPct,
				ResetAt:    existing.Quota.Window5hResetAt,
				Exhausted:  a.Exhausted5h,
				ObservedAt: existing.UpdatedAt,
			}
			snap.Windows[ratelimit.Window7d] = ratelimit.WindowState{
				PctUsed:    existing.Quota.Window7dPct,
				ResetAt:    existing.Quota.Window7dResetAt,
				Exhausted:  a.Exhausted7d,
				ObservedAt: existing.UpdatedAt,
			}
			if existing.Disabled {
				snap.Status = "disabled"
			}
		}
	}
	if _, ok := snap.Windows[ratelimit.Window5h]; !ok {
		snap.Windows[ratelimit.Window5h] = ratelimit.WindowState{ResetAt: a.ResetAt5h, Exhausted: a.Exhausted5h}
	}
	if _, ok := snap.Windows[ratelimit.Window7d]; !ok {
		snap.Windows[ratelimit.Window7d] = ratelimit.WindowState{ResetAt: a.ResetAt7d, Exhausted: a.Exhausted7d}
	}

	if h.rl.Store != nil {
		if rec, err := h.rl.Store.LastWarmup(c.Request.Context(), a.AuthID); err == nil && rec != nil {
			snap.LastWarmup = rec
		}
	}
	return snap
}

// ----------------------------------------------------------------------------
// SSE ticket plumbing
// ----------------------------------------------------------------------------

const sseTicketTTL = 60 * time.Second

type ticketEntry struct {
	expiresAt time.Time
}

var (
	ticketMu sync.Mutex
	tickets  = map[string]ticketEntry{}
)

func (h *Handler) mintTicket() (string, time.Time, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, err
	}
	tok := hex.EncodeToString(b[:])
	exp := time.Now().UTC().Add(sseTicketTTL)
	ticketMu.Lock()
	tickets[tok] = ticketEntry{expiresAt: exp}
	purgeExpiredTicketsLocked(time.Now())
	ticketMu.Unlock()
	return tok, exp, nil
}

func (h *Handler) consumeTicket(tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return false
	}
	ticketMu.Lock()
	defer ticketMu.Unlock()
	entry, ok := tickets[tok]
	if !ok {
		return false
	}
	delete(tickets, tok)
	purgeExpiredTicketsLocked(time.Now())
	return entry.expiresAt.After(time.Now())
}

func purgeExpiredTicketsLocked(now time.Time) {
	for k, v := range tickets {
		if v.expiresAt.Before(now) {
			delete(tickets, k)
		}
	}
}
