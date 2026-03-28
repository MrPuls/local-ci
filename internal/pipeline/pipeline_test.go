package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
)

type mockExecutor struct {
	executed []string
	failOn   string
}

func (m *mockExecutor) Execute(ctx context.Context, job config.JobConfig) error {
	if job.Name == m.failOn {
		return fmt.Errorf("job %s failed", job.Name)
	}
	m.executed = append(m.executed, job.Name)
	return nil
}

func (m *mockExecutor) Cleanup(ctx context.Context) error {
	return nil
}

func TestPipeline_ExecutesAllJobs(t *testing.T) {
	mock := &mockExecutor{}
	jobs := []config.JobConfig{
		{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo build"}},
		{Name: "Test", Stage: "test", Image: "alpine", Script: []string{"echo test"}},
	}
	p := NewPipeline(mock, []string{"build", "test"}, jobs)

	if err := p.Run(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(mock.executed) != 2 {
		t.Fatalf("expected 2 jobs executed, got %d", len(mock.executed))
	}
	if mock.executed[0] != "Build" || mock.executed[1] != "Test" {
		t.Errorf("unexpected execution order: %v", mock.executed)
	}
}

func TestPipeline_StopsOnFailure(t *testing.T) {
	mock := &mockExecutor{failOn: "Build"}
	jobs := []config.JobConfig{
		{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo build"}},
		{Name: "Test", Stage: "test", Image: "alpine", Script: []string{"echo test"}},
	}
	p := NewPipeline(mock, []string{"build", "test"}, jobs)

	err := p.Run(context.Background())
	if err == nil {
		t.Error("expected error when job fails")
	}

	if len(mock.executed) != 0 {
		t.Errorf("expected no jobs to complete, got %v", mock.executed)
	}
}

func TestPipeline_SecondJobFails(t *testing.T) {
	mock := &mockExecutor{failOn: "Test"}
	jobs := []config.JobConfig{
		{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo build"}},
		{Name: "Test", Stage: "test", Image: "alpine", Script: []string{"echo test"}},
		{Name: "Deploy", Stage: "deploy", Image: "alpine", Script: []string{"echo deploy"}},
	}
	p := NewPipeline(mock, []string{"build", "test", "deploy"}, jobs)

	err := p.Run(context.Background())
	if err == nil {
		t.Error("expected error when job fails")
	}

	if len(mock.executed) != 1 || mock.executed[0] != "Build" {
		t.Errorf("expected only Build to complete, got %v", mock.executed)
	}
}

func TestPipeline_EmptyJobs(t *testing.T) {
	mock := &mockExecutor{}
	p := NewPipeline(mock, []string{}, []config.JobConfig{})

	if err := p.Run(context.Background()); err != nil {
		t.Errorf("unexpected error for empty pipeline: %v", err)
	}

	if len(mock.executed) != 0 {
		t.Errorf("expected no jobs executed, got %v", mock.executed)
	}
}

func TestPipeline_ContextCancelled(t *testing.T) {
	mock := &mockExecutor{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	jobs := []config.JobConfig{
		{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo build"}},
	}
	p := NewPipeline(mock, []string{"build"}, jobs)

	// The mock doesn't check context, so this just verifies no panic.
	// A real executor would fail on the cancelled context.
	p.Run(ctx)
}
