package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
)

// fakeExecutor stands in for the Docker-backed executor so the runner's event
// stream can be exercised without a daemon.
type fakeExecutor struct {
	out  map[string]string
	fail map[string]bool
}

func (f *fakeExecutor) Execute(_ context.Context, job config.JobConfig, out io.Writer) error {
	if s, ok := f.out[job.Name]; ok {
		io.WriteString(out, s)
	}
	if f.fail != nil && f.fail[job.Name] {
		return fmt.Errorf("boom")
	}
	return nil
}

type recSink struct {
	mu     sync.Mutex
	events []Event
}

func (r *recSink) Emit(e Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

// runWith prepares and runs cfg in the given mode against fe, returning the run
// error and every emitted event.
func runWith(t *testing.T, cfg *config.Config, mode RunMode, fe *fakeExecutor) (error, []Event) {
	t.Helper()
	rec := &recSink{}
	bus := NewBus(rec)
	r := newRunner(context.Background(), cfg, bus, "run1", mode, newDiagLogger(bus, "run1"))
	if err := r.PrepareJobConfigs(RunnerOptions{}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	err := r.Run(fe)
	return err, rec.events
}

func lifecycle(events []Event) []Event {
	var out []Event
	for _, e := range events {
		switch e.Type {
		case GroupStarted, GroupFinished, JobStarted, JobFinished, LogLine:
			out = append(out, e)
		}
	}
	return out
}

func TestRunSequentialEmitsStandalone(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build"},
		Jobs:   []config.JobConfig{{Name: "a", Stage: "build", Image: "x", Script: []string{"s"}}},
	}
	err, events := runWith(t, cfg, ModeSequential, &fakeExecutor{out: map[string]string{"a": "hello\n"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	life := lifecycle(events)
	if len(life) != 3 {
		t.Fatalf("expected 3 lifecycle events, got %d: %+v", len(life), life)
	}
	if life[0].Type != JobStarted || life[0].Job != "a" || life[0].Exec != Standalone {
		t.Errorf("event 0 = %+v, want JobStarted a Standalone", life[0])
	}
	if life[1].Type != LogLine || string(life[1].Data) != "hello\n" || life[1].Exec != Standalone {
		t.Errorf("event 1 = %+v, want LogLine a Standalone 'hello'", life[1])
	}
	if life[2].Type != JobFinished || life[2].Job != "a" || life[2].Err != "" || life[2].Exec != Standalone {
		t.Errorf("event 2 = %+v, want JobFinished a Standalone no-error", life[2])
	}
}

func TestRunParallelEmitsParallelGroup(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build"},
		Jobs: []config.JobConfig{
			{Name: "a", Stage: "build", Image: "x", Script: []string{"s"}},
			{Name: "b", Stage: "build", Image: "x", Script: []string{"s"}},
		},
	}
	err, events := runWith(t, cfg, ModeParallel, &fakeExecutor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	life := lifecycle(events)
	if life[0].Type != GroupStarted || life[0].GroupKind != GroupParallelAll {
		t.Errorf("first lifecycle event = %+v, want GroupStarted ParallelAll", life[0])
	}
	if last := life[len(life)-1]; last.Type != GroupFinished || last.GroupKind != GroupParallelAll {
		t.Errorf("last lifecycle event = %+v, want GroupFinished ParallelAll", last)
	}
	for _, name := range []string{"a", "b"} {
		if !hasJobEvent(life, JobStarted, name, Concurrent) {
			t.Errorf("missing JobStarted Concurrent for %q", name)
		}
		if !hasJobEvent(life, JobFinished, name, Concurrent) {
			t.Errorf("missing JobFinished Concurrent for %q", name)
		}
	}
}

func TestRunParallelStagesEmitsStageGroupsInOrder(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build", "test"},
		Jobs: []config.JobConfig{
			{Name: "c", Stage: "build", Image: "x", Script: []string{"s"}},
			{Name: "u", Stage: "test", Image: "x", Script: []string{"s"}},
			{Name: "i", Stage: "test", Image: "x", Script: []string{"s"}},
		},
	}
	_, events := runWith(t, cfg, ModeParallelStages, &fakeExecutor{})

	var labels []string
	for _, e := range events {
		if e.Type == GroupStarted {
			if e.GroupKind != GroupStage {
				t.Errorf("group kind = %v, want GroupStage", e.GroupKind)
			}
			labels = append(labels, e.GroupLabel)
		}
	}
	if len(labels) != 2 || labels[0] != "build" || labels[1] != "test" {
		t.Errorf("stage group order = %v, want [build test]", labels)
	}
}

func TestRunSequentialMatrixEmitsMatrixGroup(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build"},
		Jobs: []config.JobConfig{
			{
				Name: "m", Stage: "build", Image: "x", Script: []string{"s"},
				Matrix: []config.MatrixEntry{{"V": {"1", "2"}}},
			},
		},
	}
	_, events := runWith(t, cfg, ModeSequential, &fakeExecutor{})

	var found bool
	for _, e := range events {
		if e.Type == GroupStarted && e.GroupKind == GroupMatrix {
			found = true
			if e.GroupLabel != "m" {
				t.Errorf("matrix group label = %q, want %q", e.GroupLabel, "m")
			}
			if len(e.Order) != 2 {
				t.Errorf("matrix group order = %v, want 2 variants", e.Order)
			}
		}
	}
	if !found {
		t.Error("no GroupMatrix event emitted")
	}
}

func TestRunDetachedEmitsDetachedAndWaitNotice(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build"},
		Jobs: []config.JobConfig{
			{Name: "a", Stage: "build", Image: "x", Script: []string{"s"}},
			{Name: "d", Stage: "build", Image: "x", Script: []string{"s"}, Parallel: ptrTrue()},
		},
	}
	_, events := runWith(t, cfg, ModeSequential, &fakeExecutor{})

	if !hasJobEvent(lifecycle(events), JobStarted, "d", Detached) {
		t.Error("missing JobStarted Detached for d")
	}
	if !hasJobEvent(lifecycle(events), JobStarted, "a", Standalone) {
		t.Error("missing JobStarted Standalone for a")
	}
	var sawNotice bool
	for _, e := range events {
		if e.Type == LogLine && e.Job == "" && strings.Contains(string(e.Data), "Waiting for detached jobs to finish") {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Error("missing 'Waiting for detached jobs to finish' notice")
	}
}

func TestRunReportsJobFailure(t *testing.T) {
	cfg := &config.Config{
		Stages: []string{"build"},
		Jobs:   []config.JobConfig{{Name: "a", Stage: "build", Image: "x", Script: []string{"s"}}},
	}
	err, events := runWith(t, cfg, ModeSequential, &fakeExecutor{fail: map[string]bool{"a": true}})
	if err == nil {
		t.Fatal("expected an error from a failing job")
	}
	var fin *Event
	for i := range events {
		if events[i].Type == JobFinished && events[i].Job == "a" {
			fin = &events[i]
		}
	}
	if fin == nil {
		t.Fatal("no JobFinished event for a")
	}
	if fin.Err == "" {
		t.Error("JobFinished.Err should be set for a failing job")
	}
	if fin.ExitCode == 0 {
		t.Error("JobFinished.ExitCode should be non-zero for a failing job")
	}
}

func hasJobEvent(events []Event, typ EventType, job string, exec ExecKind) bool {
	for _, e := range events {
		if e.Type == typ && e.Job == job && e.Exec == exec {
			return true
		}
	}
	return false
}
