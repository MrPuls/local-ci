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
	Commit      string // HEAD SHA at run start ("" outside a git repo)
	Branch      string // branch name at run start
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
	db     *sql.DB
	root   string // <xdg>/local-ci
	dbPath string // the SQLite file path
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

	s := &Store{db: db, root: root, dbPath: dbPath}
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
	if n == 0 { // fresh database: schemaSQL already created the current shape
		_, err := s.db.Exec(`INSERT INTO schema_meta (version) VALUES (?)`, schemaVersion)
		return err
	}
	var v int
	if err := s.db.QueryRow(`SELECT version FROM schema_meta LIMIT 1`).Scan(&v); err != nil {
		return err
	}
	if v < 2 {
		for _, stmt := range []string{
			`ALTER TABLE runs ADD COLUMN commit_sha TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE runs ADD COLUMN branch TEXT NOT NULL DEFAULT ''`,
		} {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate to v2: %w", err)
			}
		}
		v = 2
	}
	if v != schemaVersion {
		return fmt.Errorf("unknown schema version %d (binary supports %d)", v, schemaVersion)
	}
	_, err := s.db.Exec(`UPDATE schema_meta SET version=?`, schemaVersion)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

// Root returns the store's base directory (<xdg>/local-ci).
func (s *Store) Root() string { return s.root }

// DBPath returns the SQLite database file path.
func (s *Store) DBPath() string { return s.dbPath }

// RunDir returns the directory holding a run's log files.
func (s *Store) RunDir(id string) string { return filepath.Join(s.root, "runs", id) }

// CreateRun inserts a new run row (status should be StatusRunning).
func (s *Store) CreateRun(r Run) error {
	_, err := s.db.Exec(
		`INSERT INTO runs (id, project_path, config_path, mode, status, started_at, commit_sha, branch)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ProjectPath, r.ConfigPath, r.Mode, r.Status, r.StartedAt.UnixMilli(), r.Commit, r.Branch,
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

// ListRuns returns runs newest first, skipping offset and capped at limit. When
// all is false the list is restricted to projectPath. limit <= 0 defaults to 20.
func (s *Store) ListRuns(projectPath string, all bool, limit, offset int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	const cols = `id, project_path, config_path, mode, status, started_at, finished_at, duration_ms, error, commit_sha, branch`
	var rows *sql.Rows
	var err error
	if all {
		rows, err = s.db.Query(`SELECT `+cols+` FROM runs ORDER BY started_at DESC LIMIT ? OFFSET ?`, limit, offset)
	} else {
		rows, err = s.db.Query(`SELECT `+cols+` FROM runs WHERE project_path=? ORDER BY started_at DESC LIMIT ? OFFSET ?`, projectPath, limit, offset)
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

// CountRuns returns the total number of runs (for pagination), scoped to
// projectPath unless all is true.
func (s *Store) CountRuns(projectPath string, all bool) (int, error) {
	var n int
	var err error
	if all {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&n)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE project_path=?`, projectPath).Scan(&n)
	}
	return n, err
}

// DeleteRun removes a run, its job rows, and its on-disk log directory. A
// missing log directory is not an error.
func (s *Store) DeleteRun(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM jobs WHERE run_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM runs WHERE id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return os.RemoveAll(s.RunDir(id))
}

// OldRunIDs returns the ids of every run beyond the keep most recent (newest
// first), scoped to projectPath unless all is true — the candidates for cleanup.
func (s *Store) OldRunIDs(projectPath string, all bool, keep int) ([]string, error) {
	if keep < 0 {
		keep = 0
	}
	var rows *sql.Rows
	var err error
	if all {
		rows, err = s.db.Query(`SELECT id FROM runs ORDER BY started_at DESC LIMIT -1 OFFSET ?`, keep)
	} else {
		rows, err = s.db.Query(`SELECT id FROM runs WHERE project_path=? ORDER BY started_at DESC LIMIT -1 OFFSET ?`, projectPath, keep)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetRun returns a single run by id, or ErrNotFound.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(
		`SELECT id, project_path, config_path, mode, status, started_at, finished_at, duration_ms, error, commit_sha, branch
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
		&startedMs, &finishedMs, &durMs, &r.Error, &r.Commit, &r.Branch); err != nil {
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

// JobSample is one job execution inside the recent-runs window, ordered
// oldest-first — the raw material for duration sparklines and flakiness flags.
type JobSample struct {
	RunID     string
	Name      string
	Status    string
	StartedAt time.Time
	Duration  time.Duration
}

// JobSamples returns every job execution from the `window` most recent runs of
// projectPath (all projects when all is true), ordered oldest run first.
func (s *Store) JobSamples(projectPath string, all bool, window int) ([]JobSample, error) {
	if window <= 0 {
		window = 20
	}
	q := `
SELECT j.run_id, j.name, j.status, r.started_at, COALESCE(j.duration_ms, 0)
FROM jobs j
JOIN runs r ON r.id = j.run_id
WHERE r.id IN (SELECT id FROM runs %s ORDER BY started_at DESC LIMIT ?)
ORDER BY r.started_at ASC, j.id ASC`
	var rows *sql.Rows
	var err error
	if all {
		rows, err = s.db.Query(fmt.Sprintf(q, ""), window)
	} else {
		rows, err = s.db.Query(fmt.Sprintf(q, "WHERE project_path=?"), projectPath, window)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []JobSample
	for rows.Next() {
		var js JobSample
		var startedMs, durMs int64
		if err := rows.Scan(&js.RunID, &js.Name, &js.Status, &startedMs, &durMs); err != nil {
			return nil, err
		}
		js.StartedAt = time.UnixMilli(startedMs)
		js.Duration = time.Duration(durMs) * time.Millisecond
		out = append(out, js)
	}
	return out, rows.Err()
}
