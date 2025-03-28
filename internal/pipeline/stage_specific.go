package pipeline

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/job"
	"slices"
)

type StageSpecificPipeline struct {
	executor   Executor
	config     *config.Config
	stageNames []string
	variables  globals.Variables
}

func NewStageSpecificPipeline(executor Executor, variables globals.Variables, stageNames []string, config *config.Config) *StageSpecificPipeline {
	return &StageSpecificPipeline{
		executor:   executor,
		config:     config,
		stageNames: stageNames,
		variables:  variables,
	}
}

// Run TODO: go through jobs and find all that have the specified stage(s). If after the loop there are no jobs found - return error
func (p *StageSpecificPipeline) Run(ctx context.Context) error {
	var jobs []job.Job
	for k, v := range p.config.Jobs {
		if !slices.Contains(p.config.Stages, v.Stage) {
			return fmt.Errorf("Stage %q does not exist in file %q. ", k, p.config.FileName)
		}
		// Create the job from config
		newJob := job.NewJobConfig(s, p.config.Jobs[s], p.variables)
		jobs = append(jobs, newJob)
	}

	// Execute the jobs
	for _, j := range jobs {
		if err := p.executor.Execute(ctx, j); err != nil {
			return fmt.Errorf("job failed: %v", err)
		}
	}
	return nil
}
