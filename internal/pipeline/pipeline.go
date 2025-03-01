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

// TODO: perhaps separate configs for a general pipeline and a custom pipeline with specified jobs only

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

func NewJobSpecificPipeline(executor Executor, variables globals.Variables, jobName string) *Pipeline {
	// TODO
	return &Pipeline{}
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

		for _, j := range stageJobs {
			jobErr := p.executor.Execute(ctx, j)

			cleanupErr := p.executor.Cleanup(ctx)

			if jobErr != nil {
				if cleanupErr != nil {
					return fmt.Errorf("job %s failed: %w (cleanup also failed: %v)",
						j.GetName(), jobErr, cleanupErr)
				}
				return fmt.Errorf("job %s failed: %w", j.GetName(), jobErr)
			}

			// If only cleanup failed, report that
			if cleanupErr != nil {
				return fmt.Errorf("cleanup after job %s failed: %w", j.GetName(), cleanupErr)
			}
		}
	}
	return nil
}
