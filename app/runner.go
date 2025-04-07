package app

import (
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/pipeline"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"log"
	"slices"
)

// Runner is the main application entry point
type Runner struct {
	ctx  context.Context
	cfg  *config.Config
	jobs map[string]config.JobConfig
}

type RunnerOptions struct {
	jobNames []string
	stages   []string
}

func NewRunner(ctx context.Context, cfg *config.Config) *Runner {
	return &Runner{
		ctx:  ctx,
		cfg:  cfg,
		jobs: make(map[string]config.JobConfig),
	}
}

func (r *Runner) Run() error {
	stages := globals.NewStages(r.cfg)
	variables := globals.NewVariables(r.cfg)
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

	adapter := docker.NewConfigAdapter()
	executor := docker.NewDockerExecutor(dockerClient, adapter)

	var runErr error
	p := pipeline.NewPipeline(executor, stages, variables, r.jobs)
	runErr = p.Run(r.ctx)

	if runErr != nil {
		return runErr
	}
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
	log.Println("Starting prepare jobs...")
	for _, s := range options.stages {
		if !slices.Contains(r.cfg.Stages, s) {
			return fmt.Errorf("invalid stage %q, not present in config file: %q", s, r.cfg.FileName)
		}
	}

	if len(options.jobNames) != 0 {
		for _, j := range options.jobNames {
			if _, ok := r.cfg.Jobs[j]; ok {
				r.jobs[j] = r.cfg.Jobs[j]
			}
		}
		return nil
	}

	if len(options.stages) != 0 {
		for k, v := range r.cfg.Jobs {
			if slices.Contains(options.stages, v.Stage) {
				r.jobs[k] = v
			}
		}
		return nil
	}

	for _, s := range r.cfg.Stages {
		for k, v := range r.cfg.Jobs {
			if v.Stage == s {
				r.jobs[k] = v
			}
		}
	}
	return nil
}
