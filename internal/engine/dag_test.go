package engine

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
)

// dagFakeExecutor records execution order and fails configured jobs. failures
// maps a job name to how many attempts should fail before one succeeds
// (use a large number for "always fails").
type dagFakeExecutor struct {
	mu       sync.Mutex
	order    []string
	attempts map[string]int
	failures map[string]int
	block    map[string]time.Duration // sleep (or until ctx done) before returning
}

func (f *dagFakeExecutor) Execute(ctx context.Context, job config.JobConfig, _ io.Writer) error {
	f.mu.Lock()
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[job.Name]++
	attempt := f.attempts[job.Name]
	d := f.block[job.Name]
	f.mu.Unlock()

	if d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}

	f.mu.Lock()
	f.order = append(f.order, job.Name)
	fails := f.failures[job.Name]
	f.mu.Unlock()
	if attempt <= fails {
		return errors.New(job.Name + " failed")
	}
	return nil
}

func (f *dagFakeExecutor) ran(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[name] > 0
}

func (f *dagFakeExecutor) pos(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, n := range f.order {
		if n == name {
			return i
		}
	}
	return -1
}

func testRunner(cfg *config.Config, jobs []config.JobConfig) *Runner {
	return &Runner{
		ctx:   context.Background(),
		cfg:   cfg,
		jobs:  jobs,
		bus:   NewBus(),
		runID: "test",
		diag:  log.New(io.Discard, "", 0),
	}
}

func TestRunDAGOrder(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build", "test", "deploy"}}
	jobs := []config.JobConfig{
		{Name: "build", Stage: "build"},
		{Name: "unit", Stage: "test", Needs: config.NameList{"build"}},
		{Name: "lint", Stage: "test"}, // no needs: waits for all of stage "build"
		{Name: "ship", Stage: "deploy", Needs: config.NameList{"unit"}},
	}
	f := &dagFakeExecutor{}
	r := testRunner(cfg, jobs)

	if err := r.Run(f); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, name := range []string{"build", "unit", "lint", "ship"} {
		if !f.ran(name) {
			t.Fatalf("job %s never ran (order: %v)", name, f.order)
		}
	}
	if f.pos("build") > f.pos("unit") || f.pos("build") > f.pos("lint") {
		t.Errorf("build must finish before its dependents: %v", f.order)
	}
	if f.pos("unit") > f.pos("ship") {
		t.Errorf("unit must finish before ship: %v", f.order)
	}
}

func TestRunDAGSkipsDownstreamOfFailure(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build", "test"}}
	jobs := []config.JobConfig{
		{Name: "build", Stage: "build"},
		{Name: "unit", Stage: "test", Needs: config.NameList{"build"}},
		{Name: "after", Stage: "test", Needs: config.NameList{"unit"}},
	}
	f := &dagFakeExecutor{failures: map[string]int{"build": 99}}
	r := testRunner(cfg, jobs)

	err := r.Run(f)
	if err == nil {
		t.Fatal("Run: want error from failed build")
	}
	if !strings.Contains(err.Error(), "skipped") {
		t.Errorf("error should report skipped jobs, got: %v", err)
	}
	if f.ran("unit") || f.ran("after") {
		t.Errorf("downstream jobs ran despite failed dependency: %v", f.order)
	}
}

func TestRunDAGNeedsEarlierStageStartsWithoutBarrier(t *testing.T) {
	// "fast" needs only "build"; it must not wait for slow stage-mates.
	cfg := &config.Config{Stages: []string{"build", "test", "deploy"}}
	jobs := []config.JobConfig{
		{Name: "build", Stage: "build"},
		{Name: "slow", Stage: "test"},
		{Name: "fast", Stage: "deploy", Needs: config.NameList{"build"}},
	}
	f := &dagFakeExecutor{block: map[string]time.Duration{"slow": 300 * time.Millisecond}}
	r := testRunner(cfg, jobs)

	if err := r.Run(f); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if f.pos("fast") > f.pos("slow") {
		t.Errorf("fast (needs build only) should not wait for slow: %v", f.order)
	}
}

func TestRetrySucceedsAfterFailures(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build"}}
	jobs := []config.JobConfig{{Name: "flaky", Stage: "build", Retry: 2}}
	f := &dagFakeExecutor{failures: map[string]int{"flaky": 2}}
	r := testRunner(cfg, jobs)

	if err := r.Run(f); err != nil {
		t.Fatalf("Run: %v (attempts=%d)", err, f.attempts["flaky"])
	}
	if f.attempts["flaky"] != 3 {
		t.Errorf("attempts = %d, want 3", f.attempts["flaky"])
	}
}

func TestRetryExhausted(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build"}}
	jobs := []config.JobConfig{{Name: "broken", Stage: "build", Retry: 1}}
	f := &dagFakeExecutor{failures: map[string]int{"broken": 99}}
	r := testRunner(cfg, jobs)

	if err := r.Run(f); err == nil {
		t.Fatal("Run: want error after exhausting retries")
	}
	if f.attempts["broken"] != 2 {
		t.Errorf("attempts = %d, want 2 (1 + 1 retry)", f.attempts["broken"])
	}
}

func TestJobTimeout(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build"}}
	jobs := []config.JobConfig{{
		Name: "hang", Stage: "build",
		Timeout: config.Duration(50 * time.Millisecond),
	}}
	f := &dagFakeExecutor{block: map[string]time.Duration{"hang": 5 * time.Second}}
	r := testRunner(cfg, jobs)

	start := time.Now()
	err := r.Run(f)
	if err == nil {
		t.Fatal("Run: want timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Errorf("error should mention the timeout, got: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("timeout did not cut the job short (took %s)", time.Since(start))
	}
}

func TestDagDepsMatrixAndFiltering(t *testing.T) {
	cfg := &config.Config{Stages: []string{"build", "test"}}
	jobs := []config.JobConfig{
		{Name: "build_GO.1", Stage: "build", MatrixGroup: "build"},
		{Name: "build_GO.2", Stage: "build", MatrixGroup: "build"},
		// Inherited needs reference the matrix base name.
		{Name: "unit", Stage: "test", Needs: config.NameList{"build", "not-in-run"}},
	}
	r := testRunner(cfg, jobs)

	deps := r.dagDeps()
	got := deps["unit"]
	if len(got) != 2 || got[0] != "build_GO.1" || got[1] != "build_GO.2" {
		t.Errorf("unit deps = %v, want both build variants (filtered need dropped)", got)
	}
}
