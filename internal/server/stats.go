package server

import (
	"net/http"
	"os"
	"sort"

	"github.com/MrPuls/local-ci/internal/store"
)

// Job trend statistics: per-job duration/status history across the most
// recent runs, plus derived health signals (pass rate, flakiness). Backs the
// JOB_TRENDS panel in the history view.

type jobStatJSON struct {
	Name     string          `json:"name"`
	Samples  []jobSampleJSON `json:"samples"` // oldest first
	AvgMs    int64           `json:"avgMs"`   // mean duration of finished samples
	MaxMs    int64           `json:"maxMs"`
	PassRate float64         `json:"passRate"` // passed / finished, 0..1
	Flaky    bool            `json:"flaky"`    // both passes and failures in the window
}

type jobSampleJSON struct {
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
}

// handleJobStats aggregates the last `window` runs (default 20, capped at 100)
// into per-job trend rows, sorted by name. Scoped to this server's project by
// default — same-named jobs from other repos would corrupt the signals —
// with ?all=true to widen.
func (s *Server) handleJobStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	window := atoiDefault(q.Get("window"), 20)
	if window > 100 {
		window = 100
	}
	project, _ := os.Getwd()
	samples, err := s.store.JobSamples(project, q.Get("all") == "true", window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	byName := map[string]*jobStatJSON{}
	var names []string
	for _, sm := range samples {
		st, ok := byName[sm.Name]
		if !ok {
			st = &jobStatJSON{Name: sm.Name}
			byName[sm.Name] = st
			names = append(names, sm.Name)
		}
		st.Samples = append(st.Samples, jobSampleJSON{
			RunID: sm.RunID, Status: sm.Status, DurationMs: sm.Duration.Milliseconds(),
		})
	}

	out := make([]jobStatJSON, 0, len(names))
	sort.Strings(names)
	for _, name := range names {
		st := byName[name]
		var sum int64
		var finished, passed int64
		for _, sm := range st.Samples {
			if sm.Status == store.StatusRunning {
				continue
			}
			finished++
			sum += sm.DurationMs
			if sm.DurationMs > st.MaxMs {
				st.MaxMs = sm.DurationMs
			}
			if sm.Status == store.StatusPassed {
				passed++
			}
		}
		if finished > 0 {
			st.AvgMs = sum / finished
			st.PassRate = float64(passed) / float64(finished)
		}
		st.Flaky = passed > 0 && passed < finished
		out = append(out, *st)
	}
	writeJSON(w, http.StatusOK, map[string]any{"window": window, "jobs": out})
}
