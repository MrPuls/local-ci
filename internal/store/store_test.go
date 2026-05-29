package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "local-ci.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRunAndJobRoundTrip(t *testing.T) {
	s := openTemp(t)

	start := time.Now().Truncate(time.Millisecond)
	if err := s.CreateRun(Run{
		ID: "run1", ProjectPath: "/proj", ConfigPath: "/proj/.local-ci.yaml",
		Mode: "sequential", Status: StatusRunning, StartedAt: start,
	}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	jobID, err := s.StartJob(Job{
		RunID: "run1", Name: "build", Stage: "build", ExecKind: "standalone",
		Status: StatusRunning, StartedAt: start, LogPath: "/p/build.log",
	})
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	fin := start.Add(2 * time.Second)
	if err := s.FinishJob(jobID, StatusFailed, fin, 2*time.Second, 1, "boom"); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	if err := s.FinishRun("run1", StatusFailed, fin, 2*time.Second, "boom"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	got, err := s.GetRun("run1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != StatusFailed || got.Mode != "sequential" || got.ProjectPath != "/proj" {
		t.Errorf("run = %+v", got)
	}
	if got.Duration != 2*time.Second {
		t.Errorf("duration = %v, want 2s", got.Duration)
	}
	if got.FinishedAt.IsZero() {
		t.Error("finished_at not set")
	}

	jobs, err := s.GetJobs("run1")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	if j.Name != "build" || j.Status != StatusFailed || j.ExitCode != 1 || j.Error != "boom" || j.LogPath != "/p/build.log" {
		t.Errorf("job = %+v", j)
	}
	if j.Duration != 2*time.Second {
		t.Errorf("job duration = %v, want 2s", j.Duration)
	}
}

func TestListRunsFiltersByProjectNewestFirst(t *testing.T) {
	s := openTemp(t)
	base := time.Now().Truncate(time.Millisecond)
	mk := func(id, proj string, offset time.Duration) {
		if err := s.CreateRun(Run{ID: id, ProjectPath: proj, ConfigPath: "c", Mode: "sequential", Status: StatusPassed, StartedAt: base.Add(offset)}); err != nil {
			t.Fatalf("CreateRun %s: %v", id, err)
		}
	}
	mk("a", "/p1", 0)
	mk("b", "/p1", time.Second)
	mk("c", "/p2", 2*time.Second)

	p1, err := s.ListRuns("/p1", false, 10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(p1) != 2 || p1[0].ID != "b" || p1[1].ID != "a" {
		t.Errorf("p1 runs = %v, want [b a] newest first", ids(p1))
	}

	all, err := s.ListRuns("", true, 10)
	if err != nil {
		t.Fatalf("ListRuns all: %v", err)
	}
	if len(all) != 3 || all[0].ID != "c" {
		t.Errorf("all runs = %v, want 3 newest-first starting c", ids(all))
	}
}

func TestGetRunNotFound(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetRun("nope"); err != ErrNotFound {
		t.Errorf("GetRun = %v, want ErrNotFound", err)
	}
}

func ids(runs []Run) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.ID
	}
	return out
}
