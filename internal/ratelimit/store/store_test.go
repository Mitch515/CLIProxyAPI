package store

import (
	"context"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
)

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const id = "auth-1"
	if err := s.UpsertAccount(ctx, Account{
		AuthID:        id,
		Provider:      "claude",
		Email:         "user@example.com",
		Label:         "Personal",
		WarmupEnabled: true,
		LastSeenAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := s.RecordObservation(ctx, ratelimit.Observation{
		AuthID:     id,
		Window:     ratelimit.Window5h,
		PctUsed:    0.42,
		TokensUsed: 12345,
		ResetAt:    time.Now().UTC().Add(2 * time.Hour),
		ObservedAt: time.Now().UTC(),
		Source:     ratelimit.SourceHeader,
	}); err != nil {
		t.Fatalf("record observation: %v", err)
	}

	if err := s.UpdateExhaustion(ctx, id, ratelimit.Window5h, true, time.Now().UTC().Add(-1*time.Minute)); err != nil {
		t.Fatalf("update exhaustion: %v", err)
	}

	due, err := s.DueForWarmup(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if len(due) != 1 || due[0].AuthID != id {
		t.Fatalf("expected 1 due account got %v", due)
	}

	hist, err := s.History(ctx, id, ratelimit.Window5h, time.Now().UTC().Add(-1*time.Hour), 100)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history point got %d", len(hist))
	}
	if hist[0].PctUsed < 0.4 || hist[0].PctUsed > 0.5 {
		t.Errorf("pct used = %v", hist[0].PctUsed)
	}

	patched, err := s.Patch(ctx, id, AccountPatch{
		Label:         strPtr("Renamed"),
		WarmupEnabled: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if patched.Label != "Renamed" || patched.WarmupEnabled != false {
		t.Errorf("patch result: %+v", patched)
	}

	if err := s.RecordWarmup(ctx, WarmupRecord{
		AuthID:  id,
		FiredAt: time.Now().UTC(),
		Trigger: "manual",
		Model:   "claude-haiku-4-5",
		OK:      true,
	}); err != nil {
		t.Fatalf("record warmup: %v", err)
	}
	last, err := s.LastWarmup(ctx, id)
	if err != nil || last == nil {
		t.Fatalf("last warmup: %v / %+v", err, last)
	}
	if last.Model != "claude-haiku-4-5" || !last.OK {
		t.Errorf("warmup result: %+v", last)
	}
}

func strPtr(s string) *string  { return &s }
func boolPtr(b bool) *bool     { return &b }
