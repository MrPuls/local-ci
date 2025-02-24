package pipeline

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/job"
)

type Executor interface {
	Execute(ctx context.Context, job job.Job) error
	Cleanup(ctx context.Context) error
}

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

	for name, cfg := range jobConfigs {
		pipeline.jobs = append(pipeline.jobs, job.NewJobConfig(name, cfg, variables))
	}

	return pipeline
}

func (p *Pipeline) Run(ctx context.Context) error {
	// Execute jobs by stage
	for _, stage := range p.stages {
		// Find all jobs in this stage
		var stageJobs []job.Job
		for _, j := range p.jobs {
			if j.GetStage() == stage {
				stageJobs = append(stageJobs, j)
			}
		}

		// Execute jobs in this stage (potentially in parallel later)
		for _, j := range stageJobs {
			if err := p.executor.Execute(ctx, j); err != nil {
				return fmt.Errorf("job %s failed: %w", j.GetName(), err)
			}

			if err := p.executor.Cleanup(ctx); err != nil {
				return fmt.Errorf("cleanup after job %s failed: %w", j.GetName(), err)
			}
		}
	}

	return nil
}
