package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
)

type Pipeline struct {
	executor Executor
	jobs     []config.JobConfig
	stages   []string
}

func NewPipeline(executor Executor, stages []string, jobs []config.JobConfig) *Pipeline {
	pipeline := &Pipeline{
		executor: executor,
		stages:   stages,
		jobs:     jobs,
	}
	return pipeline
}

func (p *Pipeline) Run(ctx context.Context) error {
	log.Printf("Running jobs for stages %v", p.stages)
	log.Printf("Running jobs %v", p.jobs)
	for _, j := range p.jobs {
		if err := cmd.RunJobBootstrap(j.JobBootstrap); err != nil {
			return fmt.Errorf("Job %s bootstrap failed: %w", j.Name, err)
		}

		jobErr := p.executor.Execute(ctx, j)

		if cleanupErr := cmd.RunJobCleanup(j.JobCleanup); cleanupErr != nil {
			return fmt.Errorf("Job %s cleanup failed: %v", j.Name, cleanupErr)
		}

		if jobErr != nil {
			return fmt.Errorf("Job %s failed: %w", j.Name, jobErr)
		}
	}
	return nil
}
