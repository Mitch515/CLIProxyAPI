// Package store persists rate-limit observations and per-account warmup
// state in a single SQLite file alongside the auth-dir.
//
// We use modernc.org/sqlite (pure Go, no CGO) so cross-compilation and
// CGO_ENABLED=0 builds keep working unchanged.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// DefaultFileName is the database file written under the auth-dir.
const DefaultFileName = "cliproxy-state.db"

// Store wraps the SQLite connection and exposes typed accessors.
type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	stopRet   chan struct{}
	stopRetWG sync.WaitGroup
}

// Open opens (and migrates) the SQLite database at <dir>/cliproxy-state.db.
//
// If dir is empty an in-memory database is opened (intended for tests).
func Open(ctx context.Context, dir string) (*Store, error) {
	dsn := "file::memory:?cache=shared"
	if dir != "" {
		path := filepath.Join(dir, DefaultFileName)
		// pragmas in the DSN are honored by modernc/sqlite via _pragma=...
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ratelimit store: open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite serialises writes; single conn avoids busy errors
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ratelimit store: ping: %w", err)
	}

	s := &Store{db: db, stopRet: make(chan struct{})}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.startRetention(14 * 24 * time.Hour)
	return s, nil
}

// Close releases the underlying connection and stops the retention worker.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	close(s.stopRet)
	s.stopRetWG.Wait()
	return s.db.Close()
}

// DB returns the underlying *sql.DB, mainly for tests and ad-hoc tooling.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			auth_id        TEXT PRIMARY KEY,
			provider       TEXT NOT NULL,
			email          TEXT,
			label          TEXT,
			warmup_enabled INTEGER NOT NULL DEFAULT 1,
			warmup_model   TEXT,
			exhausted_5h   INTEGER NOT NULL DEFAULT 0,
			exhausted_7d   INTEGER NOT NULL DEFAULT 0,
			reset_at_5h    INTEGER,
			reset_at_7d    INTEGER,
			last_seen_at   INTEGER NOT NULL,
			created_at     INTEGER NOT NULL,
			updated_at     INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_provider ON accounts(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_warmup ON accounts(warmup_enabled, exhausted_5h, exhausted_7d, reset_at_5h, reset_at_7d)`,

		`CREATE TABLE IF NOT EXISTS rate_observations (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			auth_id      TEXT NOT NULL,
			observed_at  INTEGER NOT NULL,
			window       TEXT NOT NULL,
			pct_used     REAL NOT NULL,
			tokens_used  INTEGER,
			tokens_limit INTEGER,
			reset_at     INTEGER,
			source       TEXT NOT NULL,
			FOREIGN KEY(auth_id) REFERENCES accounts(auth_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_auth_window_time ON rate_observations(auth_id, window, observed_at)`,

		`CREATE TABLE IF NOT EXISTS warmup_log (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			auth_id   TEXT NOT NULL,
			fired_at  INTEGER NOT NULL,
			trigger   TEXT NOT NULL,
			model     TEXT NOT NULL,
			ok        INTEGER NOT NULL,
			error     TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_warmup_auth_time ON warmup_log(auth_id, fired_at)`,

		`INSERT OR IGNORE INTO schema_meta(k,v) VALUES('version','1')`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("ratelimit store: migrate %q: %w", firstWords(q, 6), err)
		}
	}
	return nil
}

func firstWords(s string, n int) string {
	count := 0
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			count++
			if count >= n {
				return s[:i]
			}
		}
	}
	return s
}

func (s *Store) startRetention(maxAge time.Duration) {
	s.stopRetWG.Add(1)
	go func() {
		defer s.stopRetWG.Done()
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		// Run once at start so a long-running process clears stale rows on boot.
		s.purge(maxAge)
		for {
			select {
			case <-s.stopRet:
				return
			case <-t.C:
				s.purge(maxAge)
			}
		}
	}()
}

func (s *Store) purge(maxAge time.Duration) {
	cutoff := time.Now().UTC().Add(-maxAge).Unix()
	if _, err := s.db.Exec(`DELETE FROM rate_observations WHERE observed_at < ?`, cutoff); err != nil {
		log.Debugf("ratelimit store: retention purge: %v", err)
	}
}

// Account is the in-DB row shape for the accounts table.
type Account struct {
	AuthID        string
	Provider      string
	Email         string
	Label         string
	WarmupEnabled bool
	WarmupModel   string
	Exhausted5h   bool
	Exhausted7d   bool
	ResetAt5h     time.Time
	ResetAt7d     time.Time
	LastSeenAt    time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertAccount inserts or updates the account row keyed by auth_id. Fields
// that are zero in `a` are preserved on update where it makes semantic sense
// (e.g. label/email are only overwritten if non-empty).
func (s *Store) UpsertAccount(ctx context.Context, a Account) error {
	if a.AuthID == "" {
		return errors.New("ratelimit store: empty AuthID")
	}
	now := time.Now().UTC()
	if a.LastSeenAt.IsZero() {
		a.LastSeenAt = now
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	const q = `
INSERT INTO accounts (auth_id, provider, email, label, warmup_enabled, warmup_model,
                      exhausted_5h, exhausted_7d, reset_at_5h, reset_at_7d,
                      last_seen_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(auth_id) DO UPDATE SET
  provider       = excluded.provider,
  email          = COALESCE(NULLIF(excluded.email, ''), accounts.email),
  label          = COALESCE(NULLIF(excluded.label, ''), accounts.label),
  last_seen_at   = excluded.last_seen_at,
  updated_at     = excluded.updated_at
`
	_, err := s.db.ExecContext(ctx, q,
		a.AuthID, a.Provider, a.Email, a.Label,
		boolToInt(a.WarmupEnabled), a.WarmupModel,
		boolToInt(a.Exhausted5h), boolToInt(a.Exhausted7d),
		nullableUnix(a.ResetAt5h), nullableUnix(a.ResetAt7d),
		a.LastSeenAt.Unix(), a.CreatedAt.Unix(), a.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("ratelimit store: upsert account: %w", err)
	}
	return nil
}

// GetAccount returns the row for an auth_id, or sql.ErrNoRows if missing.
func (s *Store) GetAccount(ctx context.Context, authID string) (Account, error) {
	const q = `SELECT auth_id, provider, IFNULL(email,''), IFNULL(label,''),
		warmup_enabled, IFNULL(warmup_model,''),
		exhausted_5h, exhausted_7d, IFNULL(reset_at_5h,0), IFNULL(reset_at_7d,0),
		last_seen_at, created_at, updated_at
		FROM accounts WHERE auth_id = ?`
	row := s.db.QueryRowContext(ctx, q, authID)
	return scanAccount(row)
}

// ListAccounts returns every row.
func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	const q = `SELECT auth_id, provider, IFNULL(email,''), IFNULL(label,''),
		warmup_enabled, IFNULL(warmup_model,''),
		exhausted_5h, exhausted_7d, IFNULL(reset_at_5h,0), IFNULL(reset_at_7d,0),
		last_seen_at, created_at, updated_at
		FROM accounts ORDER BY provider ASC, label ASC, auth_id ASC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PatchAccount updates user-controlled fields. Pointers being nil leave the
// existing value alone.
type AccountPatch struct {
	Label         *string
	WarmupEnabled *bool
	WarmupModel   *string
}

// Patch applies the user-supplied patch and returns the updated account.
func (s *Store) Patch(ctx context.Context, authID string, p AccountPatch) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.GetAccount(ctx, authID)
	if err != nil {
		return Account{}, err
	}
	if p.Label != nil {
		current.Label = *p.Label
	}
	if p.WarmupEnabled != nil {
		current.WarmupEnabled = *p.WarmupEnabled
	}
	if p.WarmupModel != nil {
		current.WarmupModel = *p.WarmupModel
	}
	current.UpdatedAt = time.Now().UTC()
	const q = `UPDATE accounts SET label = ?, warmup_enabled = ?, warmup_model = ?, updated_at = ? WHERE auth_id = ?`
	if _, err := s.db.ExecContext(ctx, q, current.Label, boolToInt(current.WarmupEnabled), current.WarmupModel, current.UpdatedAt.Unix(), authID); err != nil {
		return Account{}, err
	}
	return current, nil
}

// RecordObservation appends one observation row.
func (s *Store) RecordObservation(ctx context.Context, o ratelimit.Observation) error {
	if o.AuthID == "" || o.Window == "" {
		return errors.New("ratelimit store: empty AuthID/Window")
	}
	if o.ObservedAt.IsZero() {
		o.ObservedAt = time.Now().UTC()
	}
	const q = `INSERT INTO rate_observations (auth_id, observed_at, window, pct_used, tokens_used, tokens_limit, reset_at, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		o.AuthID, o.ObservedAt.UnixMilli(), string(o.Window),
		o.PctUsed, o.TokensUsed, o.TokensLimit, nullableUnix(o.ResetAt), string(o.Source))
	return err
}

// UpdateExhaustion sets the per-window exhausted flag and reset time.
func (s *Store) UpdateExhaustion(ctx context.Context, authID string, w ratelimit.Window, exhausted bool, resetAt time.Time) error {
	col := "exhausted_5h"
	rcol := "reset_at_5h"
	if w == ratelimit.Window7d {
		col = "exhausted_7d"
		rcol = "reset_at_7d"
	}
	q := fmt.Sprintf(`UPDATE accounts SET %s = ?, %s = ?, updated_at = ? WHERE auth_id = ?`, col, rcol)
	_, err := s.db.ExecContext(ctx, q, boolToInt(exhausted), nullableUnix(resetAt), time.Now().UTC().Unix(), authID)
	return err
}

// History returns observations for a window since `since`, oldest first.
func (s *Store) History(ctx context.Context, authID string, w ratelimit.Window, since time.Time, limit int) ([]ratelimit.Observation, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	const q = `SELECT observed_at, pct_used, IFNULL(tokens_used,0), IFNULL(tokens_limit,0), IFNULL(reset_at,0), source
		FROM rate_observations
		WHERE auth_id = ? AND window = ? AND observed_at >= ?
		ORDER BY observed_at ASC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, authID, string(w), since.UTC().UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ratelimit.Observation
	for rows.Next() {
		var (
			observedAt int64
			pct        float64
			used       int64
			limitTok   int64
			resetAt    int64
			source     string
		)
		if err := rows.Scan(&observedAt, &pct, &used, &limitTok, &resetAt, &source); err != nil {
			return nil, err
		}
		obs := ratelimit.Observation{
			AuthID:      authID,
			Window:      w,
			PctUsed:     pct,
			TokensUsed:  used,
			TokensLimit: limitTok,
			ObservedAt:  time.UnixMilli(observedAt).UTC(),
			Source:      ratelimit.Source(source),
		}
		if resetAt > 0 {
			obs.ResetAt = time.Unix(resetAt, 0).UTC()
		}
		out = append(out, obs)
	}
	return out, rows.Err()
}

// DueForWarmup returns accounts that are warmup-enabled and have at least
// one window flagged exhausted whose reset time has passed.
func (s *Store) DueForWarmup(ctx context.Context, now time.Time) ([]Account, error) {
	const q = `SELECT auth_id, provider, IFNULL(email,''), IFNULL(label,''),
		warmup_enabled, IFNULL(warmup_model,''),
		exhausted_5h, exhausted_7d, IFNULL(reset_at_5h,0), IFNULL(reset_at_7d,0),
		last_seen_at, created_at, updated_at
		FROM accounts
		WHERE warmup_enabled = 1
		  AND ( (exhausted_5h = 1 AND IFNULL(reset_at_5h,0) > 0 AND reset_at_5h <= ?)
		     OR (exhausted_7d = 1 AND IFNULL(reset_at_7d,0) > 0 AND reset_at_7d <= ?) )`
	t := now.UTC().Unix()
	rows, err := s.db.QueryContext(ctx, q, t, t)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecordWarmup persists one warmup attempt.
func (s *Store) RecordWarmup(ctx context.Context, w WarmupRecord) error {
	if w.FiredAt.IsZero() {
		w.FiredAt = time.Now().UTC()
	}
	const q = `INSERT INTO warmup_log (auth_id, fired_at, trigger, model, ok, error)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q, w.AuthID, w.FiredAt.Unix(), w.Trigger, w.Model, boolToInt(w.OK), w.Error)
	return err
}

// LastWarmup returns the most recent warmup record for an account, if any.
func (s *Store) LastWarmup(ctx context.Context, authID string) (*ratelimit.WarmupRecord, error) {
	const q = `SELECT fired_at, trigger, model, ok, IFNULL(error,'') FROM warmup_log WHERE auth_id = ? ORDER BY fired_at DESC LIMIT 1`
	var (
		firedAt int64
		trigger string
		model   string
		ok      int
		errMsg  string
	)
	err := s.db.QueryRowContext(ctx, q, authID).Scan(&firedAt, &trigger, &model, &ok, &errMsg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &ratelimit.WarmupRecord{
		FiredAt: time.Unix(firedAt, 0).UTC(),
		Trigger: trigger,
		Model:   model,
		OK:      ok != 0,
		Error:   errMsg,
	}, nil
}

// WarmupRecord is the input shape for RecordWarmup; it adds AuthID over the
// public ratelimit.WarmupRecord type.
type WarmupRecord struct {
	AuthID  string
	FiredAt time.Time
	Trigger string
	Model   string
	OK      bool
	Error   string
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableUnix(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Unix()
}

// scanAccount accepts both *sql.Row and *sql.Rows via the common Scan signature.
type rowScanner interface{ Scan(dest ...any) error }

func scanAccount(r rowScanner) (Account, error) {
	var (
		a            Account
		warmupEn     int
		ex5, ex7     int
		ra5, ra7     int64
		lastSeen     int64
		createdAt    int64
		updatedAt    int64
		warmupModel  string
		labelStr     string
		emailStr     string
		providerStr  string
		authIDStr    string
	)
	err := r.Scan(&authIDStr, &providerStr, &emailStr, &labelStr,
		&warmupEn, &warmupModel,
		&ex5, &ex7, &ra5, &ra7,
		&lastSeen, &createdAt, &updatedAt)
	if err != nil {
		return Account{}, err
	}
	a.AuthID = authIDStr
	a.Provider = providerStr
	a.Email = emailStr
	a.Label = labelStr
	a.WarmupEnabled = warmupEn != 0
	a.WarmupModel = warmupModel
	a.Exhausted5h = ex5 != 0
	a.Exhausted7d = ex7 != 0
	if ra5 > 0 {
		a.ResetAt5h = time.Unix(ra5, 0).UTC()
	}
	if ra7 > 0 {
		a.ResetAt7d = time.Unix(ra7, 0).UTC()
	}
	a.LastSeenAt = time.Unix(lastSeen, 0).UTC()
	a.CreatedAt = time.Unix(createdAt, 0).UTC()
	a.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return a, nil
}
