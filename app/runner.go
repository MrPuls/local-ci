package app

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type Runner struct {
	ctx  context.Context
	cfg  *config.Config
	jobs []config.JobConfig
}

type RunnerOptions struct {
	jobNames []string
	stages   []string
	env      map[string]string
	parallel bool
}

func NewRunner(ctx context.Context, cfg *config.Config) *Runner {
	return &Runner{
		ctx:  ctx,
		cfg:  cfg,
		jobs: make([]config.JobConfig, 0),
	}
}

func (r *Runner) Run() error {
	return nil
}

func (r *Runner) runSequentially() error {
	stages := r.cfg.Stages
	if len(r.jobs) == 0 {
		return fmt.Errorf("Job list is empty, nothing to run ¯\\_(ツ)_/¯\naborting... ")
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func(dockerClient *client.Client) {
		err := dockerClient.Close()
		if err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}(dockerClient)

	adapter := docker.NewConfigAdapter(r.cfg)
	executor := docker.NewDockerExecutor(dockerClient, adapter)

	log.Printf("Running jobs for stages %v", stages)
	log.Printf("Running jobs %v", r.jobs)
	for _, j := range r.jobs {
		if err := cmd.RunJobBootstrap(j.JobBootstrap, j.Variables); err != nil {
			return fmt.Errorf("Job %s bootstrap failed: %w", j.Name, err)
		}

		jobErr := executor.Execute(r.ctx, j)

		if cleanupErr := cmd.RunJobCleanup(j.JobCleanup, j.Variables); cleanupErr != nil {
			return fmt.Errorf("Job %s cleanup failed: %v", j.Name, cleanupErr)
		}

		if jobErr != nil {
			return fmt.Errorf("Job %s failed: %w", j.Name, jobErr)
		}
	}
	return nil
}

func (r *Runner) runParallel() error {
	return nil
}

func (r *Runner) Cleanup(ctx context.Context) error {
	log.Println("Starting cleanup...")
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func(dockerClient *client.Client) {
		err := dockerClient.Close()
		if err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}(dockerClient)

	cm := docker.NewContainerManager(dockerClient, nil)

	containerList, containerListError := cm.ListContainers(
		ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("label", "created_by=local-ci")),
		},
	)
	if containerListError != nil {
		return containerListError
	}

	log.Printf("Containers found: %d", len(containerList))
	if len(containerList) == 0 {
		log.Println("Nothing to cleanup ¯\\_(ツ)_/¯")
		return nil
	}

	for _, availableContainer := range containerList {
		log.Printf("Deleting container: %q, %v", availableContainer.ID, availableContainer.Names[0])
		stpErr := cm.StopContainer(ctx, availableContainer.ID, container.StopOptions{})
		if stpErr != nil {
			return stpErr
		}
		rmErr := cm.RemoveContainer(ctx, availableContainer.ID, container.RemoveOptions{})
		if rmErr != nil {
			return rmErr
		}
	}
	log.Println("All containers removed!")

	return nil
}

func (r *Runner) PrepareJobConfigs(options RunnerOptions) error {
	log.Println("Preparing jobs...")
	log.Println("Populating jobs with global variables...")
	for i, job := range r.cfg.Jobs {
		if job.Variables == nil {
			r.cfg.Jobs[i].Variables = make(map[string]string)
		}
		//local variables take precedence over global variables
		for k, v := range r.cfg.GlobalVariables {
			if _, ok := job.Variables[k]; !ok {
				r.cfg.Jobs[i].Variables[k] = v
			}
		}
	}

	// TODO: This feels bad man...

	if len(options.jobNames) != 0 {
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.jobNames, job.Name) {
				r.jobs = append(r.jobs, job)
			}
		}
		return nil
	}

	if len(options.stages) != 0 {
		for _, s := range options.stages {
			if !slices.Contains(r.cfg.Stages, s) {
				return fmt.Errorf("Requested stage %q is not present in config file: %q", s, r.cfg.FileName)
			}
		}
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.stages, job.Stage) {
				r.jobs = append(r.jobs, job)
			}
		}
		return nil
	}

	for _, s := range r.cfg.Stages {
		for _, job := range r.cfg.Jobs {
			if job.Stage == s {
				r.jobs = append(r.jobs, job)
			}
		}
	}
	return nil
}
