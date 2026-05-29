package runmanager

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/store"
)

func newManager(t *testing.T) *Manager {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st)
}

// TestTriggerStreamsAndPersists drives the manager with a fake run that blocks
// on a gate until the test has subscribed, so the live subscriber deterministically
// receives the whole event stream.
func TestTriggerStreamsAndPersists(t *testing.T) {
	m := newManager(t)
	gate := make(chan struct{})
	m.SetRunFunc(func(_ context.Context, runID string, _ engine.Spec, bus *engine.Bus) error {
		<-gate
		bus.Emit(engine.Event{Type: engine.RunStarted, RunID: runID, Mode: engine.ModeSequential, ProjectPath: "/p", ConfigPath: "c"})
		bus.Emit(engine.Event{Type: engine.JobStarted, RunID: runID, Job: "a", Exec: engine.Standalone})
		bus.Emit(engine.Event{Type: engine.JobFinished, RunID: runID, Job: "a", Exec: engine.Standalone, Duration: time.Second})
		bus.Emit(engine.Event{Type: engine.RunFinished, RunID: runID})
		return nil
	})

	id, err := m.Trigger(engine.Spec{Mode: engine.ModeSequential})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}

	ch, unsub, ok := m.Subscribe(id)
	if !ok {
		t.Fatal("expected active run to be subscribable")
	}
	defer unsub()

	close(gate)

	var types []engine.EventType
	for e := range ch {
		types = append(types, e.Type)
	}
	want := []engine.EventType{engine.RunStarted, engine.JobStarted, engine.JobFinished, engine.RunFinished}
	if len(types) != len(want) {
		t.Fatalf("stream = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("stream = %v, want %v", types, want)
		}
	}

	// The run was recorded via the recorder sink wired in by the manager.
	run, err := m.store.GetRun(id)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != store.StatusPassed {
		t.Errorf("run status = %q, want passed", run.Status)
	}
}

func TestCancelStopsRun(t *testing.T) {
	m := newManager(t)
	m.SetRunFunc(func(ctx context.Context, runID string, _ engine.Spec, bus *engine.Bus) error {
		bus.Emit(engine.Event{Type: engine.RunStarted, RunID: runID, Mode: engine.ModeSequential})
		<-ctx.Done() // block until cancelled
		bus.Emit(engine.Event{Type: engine.RunFinished, RunID: runID, Err: "cancelled"})
		return ctx.Err()
	})

	id, err := m.Trigger(engine.Spec{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}

	ch, unsub, ok := m.Subscribe(id)
	if !ok {
		t.Fatal("expected active run")
	}
	defer unsub()

	if !m.Cancel(id) {
		t.Fatal("Cancel returned false for an active run")
	}

	// Draining the stream to completion proves the run unwound after cancel.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-ch:
			if !open {
				goto done
			}
		case <-deadline:
			t.Fatal("run did not finish after cancel")
		}
	}
done:
	if m.Cancel(id) {
		t.Error("Cancel of a finished run should return false")
	}
}

func TestCancelUnknownRun(t *testing.T) {
	m := newManager(t)
	if m.Cancel("does-not-exist") {
		t.Error("Cancel of unknown run should return false")
	}
}
