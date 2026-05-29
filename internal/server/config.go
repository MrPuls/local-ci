package server

import (
	"net/http"

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
	Name         string `json:"name"`
	Stage        string `json:"stage"`
	Image        string `json:"image"`
	Parallel     bool   `json:"parallel"`
	VariantCount int    `json:"variantCount"` // >1 when the job fans out via matrix
}

// buildConfigGraph loads and validates the server's project config and shapes
// it for the UI. The path is the fixed project config (never request-derived),
// so there is no traversal surface here. Load or validation failures are
// reported as Valid:false with messages, never as a transport error.
func (s *Server) buildConfigGraph() configGraph {
	cfg := config.NewConfig(s.configPath)
	if err := cfg.LoadConfig(); err != nil {
		return configGraph{Valid: false, Path: s.configPath, Errors: []string{err.Error()}}
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
		g.Jobs = append(g.Jobs, graphJob{
			Name:         j.Name,
			Stage:        j.Stage,
			Image:        j.Image,
			Parallel:     j.IsParallel(),
			VariantCount: count,
		})
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
