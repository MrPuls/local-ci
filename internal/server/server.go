// Package server exposes the engine, run manager and store over a loopback
// JSON API plus an SSE live-event stream. It is the backend the Tauri desktop
// UI (Phase 3) talks to; in Phase 2 it is driven entirely by curl/EventSource.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/runmanager"
	"github.com/MrPuls/local-ci/internal/store"
)

// Server wires the HTTP routes to the manager and store.
type Server struct {
	store   *store.Store
	manager *runmanager.Manager
	token   string
	version string
	// configDir is the project directory the server operates in, fixed at
	// startup from the trusted --config flag. Config selection is restricted
	// to files discovery finds inside this directory, so request data never
	// contributes a path — it only picks one of the discovered names.
	configDir string
	// mu guards configPath: the selection endpoint may repoint it to another
	// discovered config file inside configDir.
	mu         sync.RWMutex
	configPath string
	// uiFS, when set (via SetUI), is the embedded SPA served for all non-/api
	// routes. Nil in API-only mode (`serve`, the dev/sidecar backend).
	uiFS fs.FS
}

// activeConfig returns the currently selected project config path.
func (s *Server) activeConfig() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configPath
}

func (s *Server) setActiveConfig(path string) {
	s.mu.Lock()
	s.configPath = path
	s.mu.Unlock()
}

// SetUI enables serving the embedded single-page app (and its assets) for every
// non-/api route. The API stays token-guarded; the UI is served unauthenticated
// (it's loopback, same-origin with the API, and carries no secrets).
func (s *Server) SetUI(f fs.FS) { s.uiFS = f }

func New(st *store.Store, mgr *runmanager.Manager, token, version, configPath string) *Server {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		abs = configPath
	}
	return &Server{
		store: st, manager: mgr, token: token, version: version,
		configPath: abs, configDir: filepath.Dir(abs),
	}
}

// safeComponent reports whether s is usable as a single path component (a run
// id or job name): non-empty, not a parent ref, and free of path separators.
// Request-supplied ids/names are validated with this before they build a file
// path, preventing traversal (CWE-22).
func safeComponent(s string) bool {
	return s != "" && s != "." && s != ".." &&
		!strings.ContainsAny(s, `/\`) && !strings.Contains(s, "..")
}

// Handler returns the fully-routed HTTP handler: a token-guarded /api surface
// and, when a UI is set, the embedded SPA served unauthenticated for all other
// routes.
func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/health", s.handleHealth)
	api.HandleFunc("GET /api/system", s.handleSystem)
	api.HandleFunc("POST /api/runs", s.handleTrigger)
	api.HandleFunc("POST /api/runs/cleanup", s.handleCleanup)
	api.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancel)
	api.HandleFunc("GET /api/runs", s.handleListRuns)
	api.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	api.HandleFunc("DELETE /api/runs/{id}", s.handleDeleteRun)
	api.HandleFunc("GET /api/runs/{id}/events", s.handleEvents)
	api.HandleFunc("GET /api/runs/{id}/log", s.handleLog)
	api.HandleFunc("GET /api/config", s.handleConfig)
	api.HandleFunc("POST /api/config/validate", s.handleValidate)
	api.HandleFunc("GET /api/config/raw", s.handleConfigRaw)
	api.HandleFunc("PUT /api/config/raw", s.handleConfigRawSave)
	api.HandleFunc("GET /api/configs", s.handleListConfigs)
	api.HandleFunc("POST /api/configs/select", s.handleSelectConfig)

	// API-only (serve / dev / Tauri sidecar): exactly the previous behaviour.
	if s.uiFS == nil {
		return s.auth(api)
	}
	// UI mode (`local-ci ui`): /api stays guarded, everything else is the SPA.
	root := http.NewServeMux()
	root.Handle("/api/", s.auth(api))
	root.Handle("/", s.uiHandler())
	return root
}

// uiHandler serves the embedded SPA: a static asset when the path matches a
// built file, otherwise index.html so a refresh or deep link still boots the
// app (the hash router then takes over).
func (s *Server) uiHandler() http.Handler {
	files := http.FileServerFS(s.uiFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if f, err := s.uiFS.Open(name); err == nil {
				f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		f, err := s.uiFS.Open("index.html")
		if err != nil {
			http.Error(w, "web UI not built", http.StatusNotImplemented)
			return
		}
		defer f.Close()
		rs, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "web UI index not seekable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, rs)
	})
}

// auth enforces the loopback bearer token. The token is also accepted as a
// ?token= query param because EventSource (SSE) cannot set request headers.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	if h := r.Header.Get("Authorization"); tokenEq(h, "Bearer "+s.token) {
		return true
	}
	return tokenEq(r.URL.Query().Get("token"), s.token)
}

func tokenEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

type systemJSON struct {
	Engine docker.Status `json:"engine"`
	DB     dbJSON        `json:"db"`
}

type dbJSON struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

// handleSystem reports the container-engine status (so the UI can show whether
// Docker/OrbStack is up and ready) and the history database's location + size.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	db := dbJSON{Path: s.store.DBPath()}
	if fi, err := os.Stat(db.Path); err == nil {
		db.SizeBytes = fi.Size()
	}
	// Under WAL mode the -wal sidecar can hold a meaningful chunk of the data.
	if fi, err := os.Stat(db.Path + "-wal"); err == nil {
		db.SizeBytes += fi.Size()
	}
	writeJSON(w, http.StatusOK, systemJSON{Engine: docker.Probe(r.Context()), DB: db})
}

type triggerRequest struct {
	Jobs   []string `json:"jobs"`
	Stages []string `json:"stages"`
	Env    []string `json:"env"`
	Remote string   `json:"remote"`
	Mode   string   `json:"mode"`
}

func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	var req triggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// The config is the server's active project config; clients select jobs /
	// stages / mode / env, never the config path.
	id, err := s.manager.Trigger(engine.Spec{
		ConfigFile: s.activeConfig(),
		JobNames:   req.Jobs,
		Stages:     req.Stages,
		Env:        req.Env,
		Remote:     req.Remote,
		Mode:       parseMode(req.Mode),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if s.manager.Cancel(r.PathValue("id")) {
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
		return
	}
	writeError(w, http.StatusNotFound, "run is not active")
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	project := q.Get("project")
	all := q.Get("all") == "true" || project == ""
	limit := atoiDefault(q.Get("limit"), 50)
	offset := atoiDefault(q.Get("offset"), 0)
	runs, err := s.store.ListRuns(project, all, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, err := s.store.CountRuns(project, all)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]runJSON, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunJSON(run, nil))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out, "total": total})
}

// handleDeleteRun removes a finished run (its row, job rows, and log files). An
// active run must be cancelled first.
func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !safeComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}
	if _, err := s.store.GetRun(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if s.manager.Active(id) {
		writeError(w, http.StatusConflict, "run is still active; cancel it first")
		return
	}
	if err := s.store.DeleteRun(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type cleanupRequest struct {
	Keep int  `json:"keep"`
	All  bool `json:"all"`
}

// handleCleanup deletes all but the `keep` most recent runs (active runs are
// always skipped). Scope follows `all`: every project, or just this one.
func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	var req cleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	project, _ := os.Getwd()
	ids, err := s.store.OldRunIDs(project, req.All, req.Keep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	deleted := 0
	for _, id := range ids {
		if s.manager.Active(id) {
			continue
		}
		if err := s.store.DeleteRun(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		deleted++
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jobs, err := s.store.GetJobs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRunJSON(run, jobs))
}

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job := r.URL.Query().Get("job")
	if job == "" {
		writeError(w, http.StatusBadRequest, "job query parameter is required (use 'pipeline' for run diagnostics)")
		return
	}
	if !safeComponent(id) || !safeComponent(job) {
		writeError(w, http.StatusBadRequest, "invalid run id or job name")
		return
	}
	name := job + ".log"
	if job == "pipeline" {
		name = "pipeline.log"
	}
	path := filepath.Join(s.store.RunDir(id), name)
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "log not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeContent(w, r, name, time.Time{}, f)
}

// --- JSON helpers and conversions ---

type runJSON struct {
	ID          string     `json:"id"`
	ProjectPath string     `json:"projectPath"`
	ConfigPath  string     `json:"configPath"`
	Mode        string     `json:"mode"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	DurationMs  int64      `json:"durationMs"`
	Error       string     `json:"error,omitempty"`
	Jobs        []jobJSON  `json:"jobs,omitempty"`
}

type jobJSON struct {
	Name       string     `json:"name"`
	Stage      string     `json:"stage"`
	ExecKind   string     `json:"execKind"`
	GroupLabel string     `json:"groupLabel,omitempty"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	DurationMs int64      `json:"durationMs"`
	ExitCode   int        `json:"exitCode"`
	Error      string     `json:"error,omitempty"`
}

func toRunJSON(r store.Run, jobs []store.Job) runJSON {
	rj := runJSON{
		ID: r.ID, ProjectPath: r.ProjectPath, ConfigPath: r.ConfigPath,
		Mode: r.Mode, Status: r.Status, StartedAt: r.StartedAt,
		FinishedAt: nilTime(r.FinishedAt), DurationMs: r.Duration.Milliseconds(), Error: r.Error,
	}
	for _, j := range jobs {
		rj.Jobs = append(rj.Jobs, jobJSON{
			Name: j.Name, Stage: j.Stage, ExecKind: j.ExecKind, GroupLabel: j.GroupLabel,
			Status: j.Status, StartedAt: nilTime(j.StartedAt), FinishedAt: nilTime(j.FinishedAt),
			DurationMs: j.Duration.Milliseconds(), ExitCode: j.ExitCode, Error: j.Error,
		})
	}
	return rj
}

func nilTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func parseMode(s string) engine.RunMode {
	switch s {
	case "parallel":
		return engine.ModeParallel
	case "parallel-stages":
		return engine.ModeParallelStages
	default:
		return engine.ModeSequential
	}
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
