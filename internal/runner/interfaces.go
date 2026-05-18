package runner

import "context"

type RunnerOptions struct {
	jobNames []string
	stages   []string
	env      map[string]string
}

type Runner interface {
	PrepareJobConfigs(options RunnerOptions) error
	RunJobs() error
	Cleanup(ctx context.Context) error
}
