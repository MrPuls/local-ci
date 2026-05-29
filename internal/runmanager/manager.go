// Package runmanager owns in-process pipeline execution for the server: it
// triggers runs (each engine.Run on its own goroutine + bus), tracks active
// runs so they can be cancelled, and fans their live events out to SSE
// subscribers. Persistence (store + event log) is wired in as sinks, so a
// server-triggered run is recorded exactly like a CLI run.
package runmanager

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/sink/eventlog"
	"github.com/MrPuls/local-ci/internal/sink/recorder"
	"github.com/MrPuls/local-ci/internal/store"
)

// runTimeout bounds a single run (matching the CLI's ceiling) so a wedged run
// can't run indefinitely on the server.
const runTimeout = time.Hour

// RunFunc executes a run. It matches engine.Run and is injectable so tests can
// drive the manager without a Docker daemon.
type RunFunc func(ctx context.Context, runID string, spec engine.Spec, bus *engine.Bus) error

type managed struct {
	cancel context.CancelFunc
	hub    *hub
}

// Manager triggers, tracks and cancels runs.
type Manager struct {
	store *store.Store
	runFn RunFunc
	mu    sync.Mutex
	runs  map[string]*managed
}

func New(st *store.Store) *Manager {
	return &Manager{
		store: st,
		runFn: engine.Run,
		runs:  make(map[string]*managed),
	}
}

// SetRunFunc overrides the executor (tests).
func (m *Manager) SetRunFunc(fn RunFunc) { m.runFn = fn }

// Trigger starts a run for spec and returns its id immediately. The run
// executes on its own goroutine; its context is owned by the manager (not any
// HTTP request) so it outlives the trigger call and is cancellable via Cancel.
func (m *Manager) Trigger(spec engine.Spec) (string, error) {
	runID := engine.NewRunID()
	runDir := m.store.RunDir(runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}

	h := newHub()
	bus := engine.NewBus(recorder.New(m.store), eventlog.New(runDir), h)
	// A generous upper bound so a wedged run (e.g. a hung image pull) can't leak
	// a goroutine + containers forever; explicit Cancel still stops it sooner.
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)

	m.mu.Lock()
	m.runs[runID] = &managed{cancel: cancel, hub: h}
	m.mu.Unlock()

	go func() {
		defer cancel()
		_ = m.runFn(ctx, runID, spec, bus)
		m.mu.Lock()
		delete(m.runs, runID)
		m.mu.Unlock()
		h.Close()
	}()

	return runID, nil
}

// Cancel cancels an active run. It returns false if the run is unknown or has
// already finished.
func (m *Manager) Cancel(runID string) bool {
	m.mu.Lock()
	mr, ok := m.runs[runID]
	m.mu.Unlock()
	if !ok {
		return false
	}
	mr.cancel()
	return true
}

// Subscribe returns a live event channel and unsubscribe func for an active
// run, or ok=false if the run is not currently running (callers then replay
// from the event log instead).
func (m *Manager) Subscribe(runID string) (<-chan engine.Event, func(), bool) {
	m.mu.Lock()
	mr, ok := m.runs[runID]
	m.mu.Unlock()
	if !ok {
		return nil, nil, false
	}
	ch, unsub := mr.hub.Subscribe()
	if ch == nil {
		return nil, nil, false
	}
	return ch, unsub, true
}

// Active reports whether a run is currently executing.
func (m *Manager) Active(runID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.runs[runID]
	return ok
}

// Shutdown cancels every active run (so the engine runs container cleanup and
// SSE streams unblock) and waits for them to clear, or until ctx expires. It is
// called on server shutdown.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	for _, mr := range m.runs {
		mr.cancel()
	}
	m.mu.Unlock()

	for {
		m.mu.Lock()
		n := len(m.runs)
		m.mu.Unlock()
		if n == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
}
