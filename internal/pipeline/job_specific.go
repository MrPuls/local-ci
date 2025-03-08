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
	config    *config.Config
	jobName   string
	variables globals.Variables
}

func NewJobSpecificPipeline(executor Executor, variables globals.Variables, jobName string, config *config.Config) *JobSpecificPipeline {
	return &JobSpecificPipeline{
		executor:  executor,
		config:    config,
		jobName:   jobName,
		variables: variables,
	}
}

func (p *JobSpecificPipeline) Run(ctx context.Context) error {
	if _, ok := p.config.Jobs[p.jobName]; !ok {
		return fmt.Errorf("Job %q does not exist in file %q. ", p.jobName, p.config.FileName)
	}
	// Create the job from config
	j := job.NewJobConfig(p.jobName, p.config.Jobs[p.jobName], p.variables)

	// Execute the job
	if err := p.executor.Execute(ctx, j); err != nil {
		return fmt.Errorf("job failed: %v", err)
	}

	return nil
}
