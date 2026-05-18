package runner

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/MrPuls/local-ci/internal/config"
)

// TODO: This should collect all jobs and run them via goroutines, reporting their progress in the terminal
// while writing all logs to files and saving them to disk and presenting the user with paths to the log files
// Probably gonna need to write a tool to write the logs to text files in each goroutine, name it and store,
// and perhaps a CLI command to clean them up after (specifically in local-ci logs dir to avoid possible abuse of command's utility)

type ParallelRunner struct {
	ctx  context.Context
	cfg  *config.Config
	jobs []config.JobConfig
}

func NewParallelRunner(ctx context.Context, cfg *config.Config) *ParallelRunner {
	return &ParallelRunner{
		ctx:  ctx,
		cfg:  cfg,
		jobs: make([]config.JobConfig, 0),
	}
}

func (pr *ParallelRunner) PrepareJobConfigs(options RunnerOptions) error {
	log.Println("Preparing jobs...")
	log.Println("Populating jobs with global variables...")
	for i, job := range pr.cfg.Jobs {
		if job.Variables == nil {
			pr.cfg.Jobs[i].Variables = make(map[string]string)
		}
		//local variables take precedence over global variables
		for k, v := range pr.cfg.GlobalVariables {
			if _, ok := job.Variables[k]; !ok {
				pr.cfg.Jobs[i].Variables[k] = v
			}
		}
	}

	// TODO: This feels bad man...

	if len(options.jobNames) != 0 {
		for _, job := range pr.cfg.Jobs {
			if slices.Contains(options.jobNames, job.Name) {
				pr.jobs = append(pr.jobs, job)
			}
		}
		return nil
	}

	if len(options.stages) != 0 {
		for _, s := range options.stages {
			if !slices.Contains(pr.cfg.Stages, s) {
				return fmt.Errorf("Requested stage %q is not present in config file: %q", s, pr.cfg.FileName)
			}
		}
		for _, job := range pr.cfg.Jobs {
			if slices.Contains(options.stages, job.Stage) {
				pr.jobs = append(pr.jobs, job)
			}
		}
		return nil
	}

	for _, s := range pr.cfg.Stages {
		for _, job := range pr.cfg.Jobs {
			if job.Stage == s {
				pr.jobs = append(pr.jobs, job)
			}
		}
	}
	return nil
}
