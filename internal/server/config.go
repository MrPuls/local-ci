package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MrPuls/local-ci/internal/config"
)

// configGraph is the shape the UI renders as a pipeline DAG.
type configGraph struct {
	Valid    bool       `json:"valid"`
	Errors   []string   `json:"errors,omitempty"`
	Path     string     `json:"path,omitempty"`
	Stages   []string   `json:"stages,omitempty"`
	Jobs     []graphJob `json:"jobs,omitempty"`
	Includes []string   `json:"includes,omitempty"`
}

type graphJob struct {
	Name         string         `json:"name"`
	Stage        string         `json:"stage"`
	Image        string         `json:"image"`
	Parallel     bool           `json:"parallel"`
	VariantCount int            `json:"variantCount"` // >1 when the job fans out via matrix
	Timeout      string         `json:"timeout,omitempty"`
	Retry        int            `json:"retry,omitempty"`
	Needs        []string       `json:"needs,omitempty"`
	Services     []graphService `json:"services,omitempty"`
	Artifacts    []string       `json:"artifacts,omitempty"`
}

type graphService struct {
	Alias string `json:"alias"`
	Image string `json:"image"`
}

// buildConfigGraph loads and validates the server's active project config and
// shapes it for the UI. The path is one of the discovered project configs
// (never request-derived), so there is no traversal surface here. Load or
// validation failures are reported as Valid:false with messages, never as a
// transport error.
func (s *Server) buildConfigGraph() configGraph {
	path := s.activeConfig()
	cfg := config.NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		return configGraph{Valid: false, Path: path, Errors: []string{err.Error()}}
	}
	if err := config.ValidateConfig(cfg); err != nil {
		return configGraph{Valid: false, Path: cfg.FileName, Stages: cfg.Stages, Errors: []string{err.Error()}}
	}

	g := configGraph{Valid: true, Path: cfg.FileName, Stages: cfg.Stages, Includes: cfg.Include}
	for _, j := range cfg.Jobs {
		variants, err := config.ExpandMatrix(j)
		count := len(variants)
		if err != nil {
			count = 0
		}
		gj := graphJob{
			Name:         j.Name,
			Stage:        j.Stage,
			Image:        j.Image,
			Parallel:     j.IsParallel(),
			VariantCount: count,
			Retry:        j.Retry,
			Needs:        j.Needs,
		}
		if j.Timeout > 0 {
			gj.Timeout = j.Timeout.Std().String()
		}
		for _, svc := range j.Services {
			gj.Services = append(gj.Services, graphService{Alias: svc.EffectiveAlias(), Image: svc.Image})
		}
		if j.Artifacts != nil {
			gj.Artifacts = j.Artifacts.Paths
		}
		g.Jobs = append(g.Jobs, gj)
	}
	return g
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.buildConfigGraph())
}

func (s *Server) handleValidate(w http.ResponseWriter, _ *http.Request) {
	g := s.buildConfigGraph()
	writeJSON(w, http.StatusOK, map[string]any{"valid": g.Valid, "errors": g.Errors})
}

// --- config discovery + selection ---

type configFileJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
	Exists bool   `json:"exists"`
}

type configListJSON struct {
	Dir     string           `json:"dir"`
	Configs []configFileJSON `json:"configs"`
}

// listConfigs discovers the config candidates in the project directory and
// marks the active one. The active config is always in the list, even when it
// doesn't match the discovery patterns (a custom --config) or doesn't exist
// yet — the UI shows it as the current, possibly missing, source.
func (s *Server) listConfigs() configListJSON {
	active := s.activeConfig()
	names, _ := config.DiscoverConfigs(s.configDir) // unreadable dir → empty list
	out := configListJSON{Dir: s.configDir}
	activeListed := false
	for _, n := range names {
		p := filepath.Join(s.configDir, n)
		isActive := p == active
		activeListed = activeListed || isActive
		out.Configs = append(out.Configs, configFileJSON{
			Name: n, Path: p, Active: isActive, Exists: true,
		})
	}
	if !activeListed {
		entry := configFileJSON{Name: filepath.Base(active), Path: active, Active: true}
		if _, err := os.Stat(active); err == nil {
			entry.Exists = true
		}
		out.Configs = append([]configFileJSON{entry}, out.Configs...)
	}
	return out
}

func (s *Server) handleListConfigs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.listConfigs())
}

// handleSelectConfig switches the active config to one of the discovered
// candidates. The request supplies only a name that must match a discovered
// entry — never a path — so selection cannot escape the project directory.
func (s *Server) handleSelectConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !safeComponent(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid config name")
		return
	}
	for _, c := range s.listConfigs().Configs {
		if c.Name == req.Name {
			s.setActiveConfig(c.Path)
			writeJSON(w, http.StatusOK, s.buildConfigGraph())
			return
		}
	}
	writeError(w, http.StatusNotFound, "no such config file")
}

// --- raw config read/write (the UI's YAML editor) ---

// maxConfigBytes caps PUT /api/config/raw bodies; a YAML config anywhere near
// 1 MiB is a mistake, and the cap keeps the editor from writing one.
const maxConfigBytes = 1 << 20

// handleConfigRaw returns the active config file verbatim. A file that doesn't
// exist yet is a 404 the editor treats as "new empty file".
func (s *Server) handleConfigRaw(w http.ResponseWriter, _ *http.Request) {
	data, err := os.ReadFile(s.activeConfig())
	if err != nil {
		writeError(w, http.StatusNotFound, "config file not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

// handleConfigRawSave atomically replaces the active config with the request
// body, then reports the validation state of what was written. Invalid YAML is
// still saved — it's the user's file and they may be mid-edit — but the
// response carries the errors so the editor can surface them.
func (s *Server) handleConfigRawSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxConfigBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "config exceeds 1 MiB limit")
		return
	}
	path := s.activeConfig()
	if err := atomicWrite(path, body); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	g := s.buildConfigGraph()
	writeJSON(w, http.StatusOK, map[string]any{
		"saved": true, "path": path, "valid": g.Valid, "errors": g.Errors,
	})
}

// atomicWrite replaces path via temp file + rename so a crash mid-write can't
// leave a truncated config. The existing file's mode is preserved.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".local-ci-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode()
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
