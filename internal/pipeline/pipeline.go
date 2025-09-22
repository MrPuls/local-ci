package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/MrPuls/local-ci/internal/config"
)

type Pipeline struct {
	executor Executor
	jobs     []config.JobConfig
	stages   []string
}

func NewPipeline(executor Executor, stages []string) *Pipeline {
	pipeline := &Pipeline{
		executor: executor,
		stages:   stages,
	}
	return pipeline
}

func (p *Pipeline) Run(ctx context.Context) error {
	log.Printf("Running jobs for stages %v", p.stages)
	log.Printf("Running jobs %v", p.jobs)
	for _, j := range p.jobs {
		jobErr := p.executor.Execute(ctx, j)

		if jobErr != nil {
			return fmt.Errorf("job %s failed: %w", j.Name, jobErr)
		}
	}
	return nil
}
