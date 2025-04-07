package pipeline

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/job"
	"log"
	"slices"
)

type JobAndStageSpecificPipeline struct {
	executor   Executor
	config     *config.Config
	stageNames []string
	variables  globals.Variables
}

func NewJobAndStageSpecificPipeline(executor Executor, variables globals.Variables, stageNames []string, config *config.Config) *JobAndStageSpecificPipeline {
	return &JobAndStageSpecificPipeline{
		executor:   executor,
		config:     config,
		stageNames: stageNames,
		variables:  variables,
	}
}

func (p *JobAndStageSpecificPipeline) Run(ctx context.Context) error {
	var jobs []job.Job
	for k, v := range p.config.Jobs {
		if slices.Contains(p.stageNames, v.Stage) {
			log.Printf("Found the job %s for stage %s", k, v.Stage)
			// Create the job from config
			newJob := job.NewJobConfig(k, p.config.Jobs[k], p.variables)
			jobs = append(jobs, newJob)
		}
	}

	if len(jobs) == 0 {
		return fmt.Errorf("No jobs were found for stages: [%v] ", p.stageNames)
	}

	// Execute the jobs
	for _, j := range jobs {
		if err := p.executor.Execute(ctx, j); err != nil {
			return fmt.Errorf("job failed: %v", err)
		}
	}
	return nil
}
