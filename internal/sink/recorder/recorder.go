// Package recorder is the persistence sink: it records the engine's event
// stream into the durable run store (SQLite metadata + per-run log files under
// <xdg>/local-ci/runs/<id>/). It runs alongside the terminal sink and never
// affects pipeline outcome — persistence failures are logged to stderr and
// swallowed.
package recorder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/store"
)

// Recorder implements engine.Sink. Bus.Emit is serialized, so the recorder is
// single-threaded and needs no locks (mirrors the terminal sink).
type Recorder struct {
	store    *store.Store
	runID    string
	runDir   string
	jobFiles map[string]*os.File
	jobIDs   map[string]int64
	pipeline *os.File
}

func New(st *store.Store) *Recorder {
	return &Recorder{
		store:    st,
		jobFiles: make(map[string]*os.File),
		jobIDs:   make(map[string]int64),
	}
}

func (r *Recorder) Emit(e engine.Event) {
	switch e.Type {
	case engine.RunStarted:
		r.onRunStarted(e)
	case engine.JobStarted:
		r.onJobStarted(e)
	case engine.LogLine:
		r.onLogLine(e)
	case engine.Diagnostic:
		r.onDiagnostic(e)
	case engine.JobFinished:
		r.onJobFinished(e)
	case engine.RunFinished:
		r.onRunFinished(e)
	}
	// GroupStarted/GroupFinished carry no durable state of their own; a job's
	// grouping is captured via its exec_kind and group_label.
}

func (r *Recorder) onRunStarted(e engine.Event) {
	r.runID = e.RunID
	r.runDir = r.store.RunDir(e.RunID)
	if err := os.MkdirAll(r.runDir, 0o755); err != nil {
		r.warn("create run dir: %v", err)
	}
	if err := r.store.CreateRun(store.Run{
		ID:          e.RunID,
		ProjectPath: e.ProjectPath,
		ConfigPath:  e.ConfigPath,
		Mode:        modeString(e.Mode),
		Status:      store.StatusRunning,
		StartedAt:   e.Time,
	}); err != nil {
		r.warn("create run: %v", err)
	}
	r.pipeline = r.openFile("pipeline.log")
}

func (r *Recorder) onJobStarted(e engine.Event) {
	logPath := filepath.Join(r.runDir, e.Job+".log")
	id, err := r.store.StartJob(store.Job{
		RunID:      r.runID,
		Name:       e.Job,
		Stage:      e.Stage,
		ExecKind:   execKindString(e.Exec),
		GroupLabel: e.GroupID,
		Status:     store.StatusRunning,
		StartedAt:  e.Time,
		LogPath:    logPath,
	})
	if err != nil {
		r.warn("start job %s: %v", e.Job, err)
		return
	}
	r.jobIDs[e.Job] = id
	r.jobFiles[e.Job] = r.openFile(e.Job + ".log")
}

func (r *Recorder) onLogLine(e engine.Event) {
	// Run-level notices (Job=="") go to the pipeline log; everything else to
	// the owning job's file.
	if e.Job == "" {
		if r.pipeline != nil {
			r.pipeline.Write(e.Data)
		}
		return
	}
	if f := r.jobFiles[e.Job]; f != nil {
		f.Write(e.Data)
	}
}

func (r *Recorder) onDiagnostic(e engine.Event) {
	if r.pipeline != nil {
		r.pipeline.Write(e.Data)
	}
}

func (r *Recorder) onJobFinished(e engine.Event) {
	if id, ok := r.jobIDs[e.Job]; ok {
		if err := r.store.FinishJob(id, statusFromErr(e.Err), e.Time, e.Duration, e.ExitCode, e.Err); err != nil {
			r.warn("finish job %s: %v", e.Job, err)
		}
		delete(r.jobIDs, e.Job)
	}
	if f, ok := r.jobFiles[e.Job]; ok {
		f.Close()
		delete(r.jobFiles, e.Job)
	}
}

func (r *Recorder) onRunFinished(e engine.Event) {
	if r.runID != "" {
		if err := r.store.FinishRun(r.runID, statusFromErr(e.Err), e.Time, e.Duration, e.Err); err != nil {
			r.warn("finish run: %v", err)
		}
	}
	for name, f := range r.jobFiles {
		f.Close()
		delete(r.jobFiles, name)
	}
	if r.pipeline != nil {
		r.pipeline.Close()
		r.pipeline = nil
	}
}

func (r *Recorder) openFile(name string) *os.File {
	if r.runDir == "" {
		return nil
	}
	f, err := os.Create(filepath.Join(r.runDir, name))
	if err != nil {
		r.warn("create %s: %v", name, err)
		return nil
	}
	return f
}

func (r *Recorder) warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "local-ci: recorder: "+format+"\n", args...)
}

func statusFromErr(errMsg string) string {
	if errMsg == "" {
		return store.StatusPassed
	}
	return store.StatusFailed
}

func modeString(m engine.RunMode) string {
	switch m {
	case engine.ModeParallel:
		return "parallel"
	case engine.ModeParallelStages:
		return "parallel-stages"
	default:
		return "sequential"
	}
}

func execKindString(k engine.ExecKind) string {
	switch k {
	case engine.Concurrent:
		return "concurrent"
	case engine.Detached:
		return "detached"
	default:
		return "standalone"
	}
}
