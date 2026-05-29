// Package store is the durable run-history layer: an embedded SQLite database
// (pure-Go modernc driver) plus a central directory of per-run log files. It is
// written by the persistence sink during a run and read by the `local-ci runs`
// / `local-ci log` commands (and later the server/UI).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MrPuls/local-ci/internal/integrations/fs"
	_ "modernc.org/sqlite"
)

// Run statuses.
const (
	StatusRunning = "running"
	StatusPassed  = "passed"
	StatusFailed  = "failed"
)

// ErrNotFound is returned by GetRun when no run matches the id.
var ErrNotFound = errors.New("run not found")

// Run is a persisted pipeline run. Times are zero and Duration is 0 until the
// run finishes.
type Run struct {
	ID          string
	ProjectPath string
	ConfigPath  string
	Mode        string
	Status      string
	StartedAt   time.Time
	FinishedAt  time.Time
	Duration    time.Duration
	Error       string
}

// Job is a persisted job within a run.
type Job struct {
	ID         int64
	RunID      string
	Name       string
	Stage      string
	ExecKind   string
	GroupLabel string
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	Duration   time.Duration
	ExitCode   int
	Error      string
	LogPath    string
}

// Store wraps the SQLite database and the run-log root directory.
type Store struct {
	db   *sql.DB
	root string // <xdg>/local-ci
}

// DefaultDBPath returns the standard database path under the XDG data dir.
func DefaultDBPath() (string, error) {
	dir, err := fs.GetDefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "local-ci.db"), nil
}

// Open opens (creating if needed) the store at dbPath, applying the schema.
// The parent directory is created quietly; it intentionally does not use
// fs.MakeDefaultDir, which prints to stdout.
func Open(dbPath string) (*Store, error) {
	root := filepath.Dir(dbPath)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection keeps per-connection pragmas effective and serializes
	// writes for this process; the bus already feeds the sink serially.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db, root: root}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_meta`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`INSERT INTO schema_meta (version) VALUES (?)`, schemaVersion); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// Root returns the store's base directory (<xdg>/local-ci).
func (s *Store) Root() string { return s.root }

// RunDir returns the directory holding a run's log files.
func (s *Store) RunDir(id string) string { return filepath.Join(s.root, "runs", id) }

// CreateRun inserts a new run row (status should be StatusRunning).
func (s *Store) CreateRun(r Run) error {
	_, err := s.db.Exec(
		`INSERT INTO runs (id, project_path, config_path, mode, status, started_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.ProjectPath, r.ConfigPath, r.Mode, r.Status, r.StartedAt.UnixMilli(),
	)
	return err
}

// FinishRun records a run's terminal status, finish time and duration.
func (s *Store) FinishRun(id, status string, finishedAt time.Time, dur time.Duration, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE runs SET status=?, finished_at=?, duration_ms=?, error=? WHERE id=?`,
		status, finishedAt.UnixMilli(), dur.Milliseconds(), errMsg, id,
	)
	return err
}

// StartJob inserts a job row and returns its rowid.
func (s *Store) StartJob(j Job) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO jobs (run_id, name, stage, exec_kind, group_label, status, started_at, log_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.RunID, j.Name, j.Stage, j.ExecKind, j.GroupLabel, j.Status, j.StartedAt.UnixMilli(), j.LogPath,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishJob records a job's terminal status, finish time, duration and exit code.
func (s *Store) FinishJob(id int64, status string, finishedAt time.Time, dur time.Duration, exitCode int, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status=?, finished_at=?, duration_ms=?, exit_code=?, error=? WHERE id=?`,
		status, finishedAt.UnixMilli(), dur.Milliseconds(), exitCode, errMsg, id,
	)
	return err
}

// ListRuns returns the most recent runs, newest first. When all is false the
// list is restricted to projectPath. limit <= 0 defaults to 20.
func (s *Store) ListRuns(projectPath string, all bool, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	const cols = `id, project_path, config_path, mode, status, started_at, finished_at, duration_ms, error`
	var rows *sql.Rows
	var err error
	if all {
		rows, err = s.db.Query(`SELECT `+cols+` FROM runs ORDER BY started_at DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`SELECT `+cols+` FROM runs WHERE project_path=? ORDER BY started_at DESC LIMIT ?`, projectPath, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRun returns a single run by id, or ErrNotFound.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(
		`SELECT id, project_path, config_path, mode, status, started_at, finished_at, duration_ms, error
		 FROM runs WHERE id=?`, id,
	)
	r, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return r, err
}

// GetJobs returns a run's jobs in insertion order.
func (s *Store) GetJobs(runID string) ([]Job, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, name, stage, exec_kind, group_label, status, started_at, finished_at, duration_ms, exit_code, error, log_path
		 FROM jobs WHERE run_id=? ORDER BY id`, runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRun(sc scanner) (Run, error) {
	var r Run
	var startedMs int64
	var finishedMs, durMs sql.NullInt64
	if err := sc.Scan(&r.ID, &r.ProjectPath, &r.ConfigPath, &r.Mode, &r.Status,
		&startedMs, &finishedMs, &durMs, &r.Error); err != nil {
		return Run{}, err
	}
	r.StartedAt = time.UnixMilli(startedMs)
	if finishedMs.Valid {
		r.FinishedAt = time.UnixMilli(finishedMs.Int64)
	}
	if durMs.Valid {
		r.Duration = time.Duration(durMs.Int64) * time.Millisecond
	}
	return r, nil
}

func scanJob(sc scanner) (Job, error) {
	var j Job
	var startedMs, finishedMs, durMs, exit sql.NullInt64
	if err := sc.Scan(&j.ID, &j.RunID, &j.Name, &j.Stage, &j.ExecKind, &j.GroupLabel, &j.Status,
		&startedMs, &finishedMs, &durMs, &exit, &j.Error, &j.LogPath); err != nil {
		return Job{}, err
	}
	if startedMs.Valid {
		j.StartedAt = time.UnixMilli(startedMs.Int64)
	}
	if finishedMs.Valid {
		j.FinishedAt = time.UnixMilli(finishedMs.Int64)
	}
	if durMs.Valid {
		j.Duration = time.Duration(durMs.Int64) * time.Millisecond
	}
	if exit.Valid {
		j.ExitCode = int(exit.Int64)
	}
	return j, nil
}
