package app

import (
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
)

func TestJobsByStage(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Stages: []string{"build", "test", "deploy"},
		},
		jobs: []config.JobConfig{
			{Name: "compile", Stage: "build"},
			{Name: "unit", Stage: "test"},
			{Name: "integration", Stage: "test"},
		},
	}

	groups := r.jobsByStage()

	// "deploy" has no jobs and must be omitted.
	if len(groups) != 2 {
		t.Fatalf("expected 2 stage groups, got %d", len(groups))
	}

	// Stage order must follow cfg.Stages.
	if groups[0].stage != "build" {
		t.Errorf("expected first group stage %q, got %q", "build", groups[0].stage)
	}
	if groups[1].stage != "test" {
		t.Errorf("expected second group stage %q, got %q", "test", groups[1].stage)
	}

	if got := jobNames(groups[0].jobs); len(got) != 1 || got[0] != "compile" {
		t.Errorf("unexpected build jobs: %v", got)
	}
	// Job order within a stage must be preserved.
	if got := jobNames(groups[1].jobs); len(got) != 2 || got[0] != "unit" || got[1] != "integration" {
		t.Errorf("unexpected test jobs: %v", got)
	}
}

func TestJobsByStageNoJobs(t *testing.T) {
	r := &Runner{
		cfg:  &config.Config{Stages: []string{"build"}},
		jobs: nil,
	}
	if groups := r.jobsByStage(); len(groups) != 0 {
		t.Errorf("expected no stage groups, got %d", len(groups))
	}
}
