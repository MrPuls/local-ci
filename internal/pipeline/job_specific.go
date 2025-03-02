package pipeline

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/job"
)

type JobSpecificPipeline struct {
	executor  Executor
	jobConfig config.JobConfig
	jobName   string
	variables globals.Variables
}

func NewJobSpecificPipeline(executor Executor, variables globals.Variables, jobName string, jobConfig config.JobConfig) *JobSpecificPipeline {
	return &JobSpecificPipeline{
		executor:  executor,
		jobConfig: jobConfig,
		jobName:   jobName,
		variables: variables,
	}
}

func (p *JobSpecificPipeline) Run(ctx context.Context) error {

	// Create the job from config
	j := job.NewJobConfig(p.jobName, p.jobConfig, p.variables)

	// Execute the job
	if err := p.executor.Execute(ctx, j); err != nil {
		// Always try to clean up
		cleanupErr := p.executor.Cleanup(ctx)
		if cleanupErr != nil {
			return fmt.Errorf("job failed: %v (cleanup also failed: %v)", err, cleanupErr)
		}
		return fmt.Errorf("job failed: %v", err)
	}

	// Clean up after successful execution
	if err := p.executor.Cleanup(ctx); err != nil {
		return fmt.Errorf("cleanup failed: %v", err)
	}

	return nil
}
