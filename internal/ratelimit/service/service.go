// Package service is the in-process coordinator for the rate-limit
// subsystem. It receives observations from the executor publisher (via the
// global ratelimit.Bus), persists them into SQLite, mirrors window state
// onto in-memory Auth records, and broadcasts live update events to any
// SSE subscriber.
package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/store"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// EventKind enumerates the SSE event types emitted by the service.
type EventKind string

const (
	EventAccountChanged EventKind = "account.changed"
	EventWarmupFired    EventKind = "warmup.fired"
)

// Event is one item broadcast to SSE subscribers.
type Event struct {
	Kind EventKind   `json:"kind"`
	At   time.Time   `json:"at"`
	Data interface{} `json:"data"`
}

// Service buffers observations on a channel, persists them, and fans out
// live events. It is started once at process boot via Start(ctx) and stopped
// via Stop().
type Service struct {
	store       *store.Store
	coreManager *coreauth.Manager

	in       chan ratelimit.Observation
	stopOnce sync.Once
	stopped  chan struct{}

	subsMu sync.RWMutex
	subs   map[uint64]chan Event
	nextID atomic.Uint64

	// reconcilerInterval controls how often we sync new auths from coreManager.
	reconcilerInterval time.Duration
}

// New constructs a Service. The caller must invoke Start to begin processing.
func New(s *store.Store, cm *coreauth.Manager) *Service {
	return &Service{
		store:              s,
		coreManager:        cm,
		in:                 make(chan ratelimit.Observation, 1024),
		stopped:            make(chan struct{}),
		subs:               make(map[uint64]chan Event),
		reconcilerInterval: 60 * time.Second,
	}
}

// Submit pushes an observation onto the internal channel. It is the function
// that the executor publisher calls via ratelimit.SetSink.
func (s *Service) Submit(o ratelimit.Observation) {
	if s == nil {
		return
	}
	select {
	case s.in <- o:
	default:
		// Drop on backpressure; the dashboard tolerates missing samples.
		log.Debugf("ratelimit service: observation channel full, dropping AuthID=%s window=%s", o.AuthID, o.Window)
	}
}

// Start launches the worker goroutines and registers Submit as the global sink.
// Call once at process boot.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	ratelimit.SetSink(s.Submit)
	go s.runDrain(ctx)
	go s.runReconciler(ctx)
}

// Stop closes the channel and unregisters the sink. It is idempotent.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		ratelimit.SetSink(nil)
		close(s.stopped)
	})
}

func (s *Service) runDrain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopped:
			return
		case o := <-s.in:
			s.processOne(ctx, o)
		}
	}
}

func (s *Service) processOne(ctx context.Context, o ratelimit.Observation) {
	if o.AuthID == "" {
		return
	}

	// Make sure the account row exists so the foreign key on observations holds.
	acc := store.Account{
		AuthID:        o.AuthID,
		Provider:      o.Provider,
		Email:         o.Email,
		Label:         o.Label,
		WarmupEnabled: true,
		LastSeenAt:    o.ObservedAt,
	}
	if err := s.store.UpsertAccount(ctx, acc); err != nil {
		log.Debugf("ratelimit service: upsert account failed: %v", err)
	}

	if err := s.store.RecordObservation(ctx, o); err != nil {
		log.Debugf("ratelimit service: record observation failed: %v", err)
	}

	exhausted := o.Exhausted || o.PctUsed >= 1.0
	if exhausted {
		if err := s.store.UpdateExhaustion(ctx, o.AuthID, o.Window, true, o.ResetAt); err != nil {
			log.Debugf("ratelimit service: update exhaustion failed: %v", err)
		}
	} else if o.PctUsed > 0 {
		// We don't clear exhaustion from an in-progress observation. It is
		// cleared only when the warmup ping reports a fresh window (low pct).
	}

	s.mirrorOntoAuth(ctx, o, exhausted)
	s.broadcastAccountChanged(o.AuthID)
}

func (s *Service) mirrorOntoAuth(ctx context.Context, o ratelimit.Observation, exhausted bool) {
	if s.coreManager == nil {
		return
	}
	existing, ok := s.coreManager.GetByID(o.AuthID)
	if !ok || existing == nil {
		return
	}
	clone := existing.Clone()
	switch o.Window {
	case ratelimit.Window5h:
		clone.Quota.Window5hPct = o.PctUsed
		if !o.ResetAt.IsZero() {
			clone.Quota.Window5hResetAt = o.ResetAt
		}
	case ratelimit.Window7d:
		clone.Quota.Window7dPct = o.PctUsed
		if !o.ResetAt.IsZero() {
			clone.Quota.Window7dResetAt = o.ResetAt
		}
	}
	if exhausted {
		clone.Quota.Exceeded = true
		clone.Quota.Reason = "rate_limit_window_full"
		if !o.ResetAt.IsZero() && (clone.Quota.NextRecoverAt.IsZero() || o.ResetAt.After(clone.Quota.NextRecoverAt)) {
			clone.Quota.NextRecoverAt = o.ResetAt
		}
	}
	if _, err := s.coreManager.Update(ctx, clone); err != nil {
		log.Debugf("ratelimit service: update auth %s failed: %v", o.AuthID, err)
	}
}

// runReconciler periodically pulls auths from coreManager and ensures every
// one has a corresponding row in the accounts table. New OAuth logins surface
// in the dashboard within one cycle without needing to touch login flows.
func (s *Service) runReconciler(ctx context.Context) {
	t := time.NewTicker(s.reconcilerInterval)
	defer t.Stop()
	s.reconcileOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopped:
			return
		case <-t.C:
			s.reconcileOnce(ctx)
		}
	}
}

func (s *Service) reconcileOnce(ctx context.Context) {
	if s.coreManager == nil {
		return
	}
	for _, a := range s.coreManager.List() {
		if a == nil || a.ID == "" {
			continue
		}
		_, email := a.AccountInfo()
		acc := store.Account{
			AuthID:        a.ID,
			Provider:      a.Provider,
			Email:         email,
			Label:         a.Label,
			WarmupEnabled: true,
			LastSeenAt:    time.Now().UTC(),
		}
		if err := s.store.UpsertAccount(ctx, acc); err != nil {
			log.Debugf("ratelimit service: reconcile upsert: %v", err)
		}
	}
}

// Subscribe registers a new SSE listener. The returned cancel function must
// be called when the subscriber goes away to free its buffered channel.
func (s *Service) Subscribe() (<-chan Event, func()) {
	id := s.nextID.Add(1)
	ch := make(chan Event, 16)
	s.subsMu.Lock()
	s.subs[id] = ch
	s.subsMu.Unlock()
	return ch, func() {
		s.subsMu.Lock()
		if existing, ok := s.subs[id]; ok {
			delete(s.subs, id)
			close(existing)
		}
		s.subsMu.Unlock()
	}
}

// PublishWarmup is called by the warmup scheduler so subscribers see the event live.
func (s *Service) PublishWarmup(record ratelimit.WarmupRecord, authID string) {
	s.broadcast(Event{Kind: EventWarmupFired, At: time.Now().UTC(), Data: map[string]any{
		"id":     authID,
		"record": record,
	}})
}

func (s *Service) broadcastAccountChanged(authID string) {
	s.broadcast(Event{Kind: EventAccountChanged, At: time.Now().UTC(), Data: map[string]any{"id": authID}})
}

func (s *Service) broadcast(e Event) {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()
	for _, ch := range s.subs {
		select {
		case ch <- e:
		default:
			// Slow subscriber; drop event for them.
		}
	}
}
