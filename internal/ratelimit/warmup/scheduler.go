// Package warmup fires a tiny prompt against any account whose 5h or 7d
// window has just reset, so the next window starts at the earliest possible
// second. This avoids the "wasted hours" problem where an exhausted account
// silently waits until the user happens to hit it again.
package warmup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/service"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/store"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
)

// Scheduler periodically scans the store for accounts due for warmup and
// fires a one-token request through the proxy's own execution path. Calling
// the executor directly (rather than going through HTTP) means the same
// response-header-parsing pipeline updates the new window state on success.
type Scheduler struct {
	store       *store.Store
	coreManager *coreauth.Manager
	service     *service.Service

	tickInterval time.Duration

	stopOnce sync.Once
	stopped  chan struct{}

	enabled bool
}

// Options configures the scheduler.
type Options struct {
	// TickInterval controls how often the store is scanned. Default 30s.
	TickInterval time.Duration
	// Enabled toggles the entire scheduler. When false, FireOnce is still
	// callable for manual warmup but the periodic tick is suppressed.
	Enabled bool
}

// New constructs a Scheduler. Start launches the tick goroutine.
func New(s *store.Store, cm *coreauth.Manager, svc *service.Service, opt Options) *Scheduler {
	tick := opt.TickInterval
	if tick <= 0 {
		tick = 30 * time.Second
	}
	return &Scheduler{
		store:        s,
		coreManager:  cm,
		service:      svc,
		tickInterval: tick,
		stopped:      make(chan struct{}),
		enabled:      opt.Enabled,
	}
}

// Start runs the background tick goroutine. Idempotent.
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || !s.enabled {
		return
	}
	go s.run(ctx)
}

// Stop terminates the scheduler. Idempotent.
func (s *Scheduler) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopped) })
}

func (s *Scheduler) run(ctx context.Context) {
	t := time.NewTicker(s.tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopped:
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	due, err := s.store.DueForWarmup(ctx, time.Now().UTC())
	if err != nil {
		log.Debugf("warmup: due query failed: %v", err)
		return
	}
	for _, acc := range due {
		if err := s.FireOnce(ctx, acc.AuthID, false); err != nil {
			log.Debugf("warmup: auto fire %s failed: %v", acc.AuthID, err)
		}
	}
}

// FireOnce executes one warmup ping for the given account. The trigger flag
// distinguishes manual UI clicks from the automatic tick. Returns an error
// only when the call could not be attempted; a 4xx/5xx upstream still counts
// as "fired" and is logged with ok=false.
func (s *Scheduler) FireOnce(ctx context.Context, authID string, manual bool) error {
	if s == nil {
		return errors.New("warmup: scheduler is nil")
	}
	auth, ok := s.coreManager.GetByID(authID)
	if !ok || auth == nil {
		return fmt.Errorf("warmup: auth %s not found", authID)
	}
	provider := strings.ToLower(strings.TrimSpace(auth.Provider))
	executor, ok := s.coreManager.Executor(provider)
	if !ok || executor == nil {
		return fmt.Errorf("warmup: no executor registered for provider %s", provider)
	}

	dbAcc, _ := s.store.GetAccount(ctx, authID)
	model := PickModel(provider, dbAcc.WarmupModel)
	if model == "" {
		return fmt.Errorf("warmup: no default model for provider %s", provider)
	}

	payload, format := buildPayload(provider, model)

	req := cliproxyexecutor.Request{
		Model:   model,
		Payload: payload,
		Format:  format,
	}
	opts := cliproxyexecutor.Options{
		Stream: false,
		Metadata: map[string]any{
			cliproxyexecutor.PinnedAuthMetadataKey: authID,
		},
	}

	trigger := "auto"
	if manual {
		trigger = "manual"
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Stash the selected auth so the executor logging helper can publish
	// observations from this warmup ping back into the dashboard pipeline,
	// which is what naturally clears the exhausted flag on success.
	callCtx = context.WithValue(callCtx, "cliproxy.selected_auth", auth)
	_, execErr := executor.Execute(callCtx, auth, req, opts)

	rec := store.WarmupRecord{
		AuthID:  authID,
		FiredAt: time.Now().UTC(),
		Trigger: trigger,
		Model:   model,
		OK:      execErr == nil,
	}
	if execErr != nil {
		rec.Error = execErr.Error()
	}
	if err := s.store.RecordWarmup(ctx, rec); err != nil {
		log.Debugf("warmup: log insert failed: %v", err)
	}
	if s.service != nil {
		s.service.PublishWarmup(ratelimit.WarmupRecord{
			FiredAt: rec.FiredAt,
			Trigger: rec.Trigger,
			Model:   rec.Model,
			OK:      rec.OK,
			Error:   rec.Error,
		}, authID)
	}
	return nil
}
