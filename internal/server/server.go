// Package server exposes the engine, run manager and store over a loopback
// JSON API plus an SSE live-event stream. It is the backend the Tauri desktop
// UI (Phase 3) talks to; in Phase 2 it is driven entirely by curl/EventSource.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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
}

func New(st *store.Store, mgr *runmanager.Manager, token, version string) *Server {
	return &Server{store: st, manager: mgr, token: token, version: version}
}

// Handler returns the fully-routed, auth-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/runs", s.handleTrigger)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancel)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /api/runs/{id}/log", s.handleLog)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/config/validate", s.handleValidate)
	return s.auth(mux)
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

type triggerRequest struct {
	ConfigFile string   `json:"configFile"`
	Jobs       []string `json:"jobs"`
	Stages     []string `json:"stages"`
	Env        []string `json:"env"`
	Remote     string   `json:"remote"`
	Mode       string   `json:"mode"`
}

func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	var req triggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	configFile := req.ConfigFile
	if configFile == "" {
		configFile = ".local-ci.yaml"
	}
	id, err := s.manager.Trigger(engine.Spec{
		ConfigFile: configFile,
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
	runs, err := s.store.ListRuns(project, all, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]runJSON, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunJSON(run, nil))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
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
	job := r.URL.Query().Get("job")
	if job == "" {
		writeError(w, http.StatusBadRequest, "job query parameter is required (use 'pipeline' for run diagnostics)")
		return
	}
	name := job + ".log"
	if job == "pipeline" {
		name = "pipeline.log"
	}
	path := filepath.Join(s.store.RunDir(r.PathValue("id")), name)
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
