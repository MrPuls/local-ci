package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestRecorderPersistsRunJobsAndLogs(t *testing.T) {
	st := openStore(t)
	r := New(st)

	t0 := time.Now().Truncate(time.Millisecond)
	t1 := t0.Add(3 * time.Second)

	r.Emit(engine.Event{Type: engine.RunStarted, RunID: "r1", Time: t0, Mode: engine.ModeSequential,
		ProjectPath: "/proj", ConfigPath: "/proj/.local-ci.yaml", Order: []string{"a"}})
	r.Emit(engine.Event{Type: engine.JobStarted, RunID: "r1", Time: t0, Job: "a", Stage: "build", Exec: engine.Standalone})
	r.Emit(engine.Event{Type: engine.LogLine, RunID: "r1", Job: "a", Exec: engine.Standalone, Data: []byte("hello\n")})
	r.Emit(engine.Event{Type: engine.JobFinished, RunID: "r1", Time: t1, Job: "a", Exec: engine.Standalone, Duration: 3 * time.Second, ExitCode: 0})
	r.Emit(engine.Event{Type: engine.RunFinished, RunID: "r1", Time: t1, Duration: 3 * time.Second})

	run, err := st.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != store.StatusPassed || run.Mode != "sequential" || run.ProjectPath != "/proj" {
		t.Errorf("run = %+v", run)
	}
	if run.Duration != 3*time.Second {
		t.Errorf("run duration = %v, want 3s", run.Duration)
	}

	jobs, err := st.GetJobs("r1")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "a" || jobs[0].Status != store.StatusPassed || jobs[0].ExecKind != "standalone" {
		t.Fatalf("jobs = %+v", jobs)
	}

	logData, err := os.ReadFile(filepath.Join(st.RunDir("r1"), "a.log"))
	if err != nil {
		t.Fatalf("read job log: %v", err)
	}
	if string(logData) != "hello\n" {
		t.Errorf("a.log = %q, want %q", logData, "hello\n")
	}
}

func TestRecorderRecordsFailureAndDiagnostics(t *testing.T) {
	st := openStore(t)
	r := New(st)
	now := time.Now()

	r.Emit(engine.Event{Type: engine.RunStarted, RunID: "r2", Time: now, Mode: engine.ModeParallel, ProjectPath: "/p", ConfigPath: "c"})
	r.Emit(engine.Event{Type: engine.Diagnostic, RunID: "r2", Data: []byte("setup\n")})
	r.Emit(engine.Event{Type: engine.JobStarted, RunID: "r2", Time: now, Job: "j", Exec: engine.Concurrent})
	r.Emit(engine.Event{Type: engine.JobFinished, RunID: "r2", Time: now, Job: "j", Exec: engine.Concurrent, Duration: time.Second, ExitCode: 1, Err: "Job j failed: boom"})
	r.Emit(engine.Event{Type: engine.RunFinished, RunID: "r2", Time: now, Err: "Job j failed: boom"})

	run, err := st.GetRun("r2")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != store.StatusFailed {
		t.Errorf("run status = %q, want failed", run.Status)
	}
	if run.Mode != "parallel" {
		t.Errorf("run mode = %q, want parallel", run.Mode)
	}

	jobs, _ := st.GetJobs("r2")
	if len(jobs) != 1 || jobs[0].Status != store.StatusFailed || jobs[0].ExitCode != 1 {
		t.Fatalf("jobs = %+v", jobs)
	}

	pipe, err := os.ReadFile(filepath.Join(st.RunDir("r2"), "pipeline.log"))
	if err != nil {
		t.Fatalf("read pipeline.log: %v", err)
	}
	if string(pipe) != "setup\n" {
		t.Errorf("pipeline.log = %q, want %q", pipe, "setup\n")
	}
}
