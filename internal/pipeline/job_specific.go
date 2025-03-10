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
	jobNames  []string
	variables globals.Variables
}

func NewJobSpecificPipeline(executor Executor, variables globals.Variables, jobNames []string, config *config.Config) *JobSpecificPipeline {
	return &JobSpecificPipeline{
		executor:  executor,
		config:    config,
		jobNames:  jobNames,
		variables: variables,
	}
}

func (p *JobSpecificPipeline) Run(ctx context.Context) error {
	var jobs []job.Job
	for _, j := range p.jobNames {
		if _, ok := p.config.Jobs[j]; !ok {
			return fmt.Errorf("Job %q does not exist in file %q. ", j, p.config.FileName)
		}
		// Create the job from config
		newJob := job.NewJobConfig(j, p.config.Jobs[j], p.variables)
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
