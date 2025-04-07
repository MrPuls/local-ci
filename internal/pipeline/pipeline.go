package pipeline

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/job"
	"log"
)

type Pipeline struct {
	executor  Executor
	jobs      []job.Job
	stages    []string
	variables map[string]string
}

func NewPipeline(executor Executor, stages globals.Stages, variables globals.Variables, jobConfigs map[string]config.JobConfig) *Pipeline {
	pipeline := &Pipeline{
		executor:  executor,
		stages:    stages.GetStages(),
		variables: variables.GetGlobalVariables(),
	}

	// TODO: Think there is a way to handle it better
	for name, cfg := range jobConfigs {
		pipeline.jobs = append(pipeline.jobs, job.NewJobConfig(name, cfg, variables))
	}

	return pipeline
}

func (p *Pipeline) Run(ctx context.Context) error {
	log.Printf("Running jobs for stages %v", p.stages)
	log.Printf("Running jobs %v", p.jobs)
	// TODO: Jobs run in a different order each time, must be deterministic instead
	for _, j := range p.jobs {
		jobErr := p.executor.Execute(ctx, j)

		if jobErr != nil {
			return fmt.Errorf("job %s failed: %w", j.GetName(), jobErr)
		}
	}
	return nil
}
