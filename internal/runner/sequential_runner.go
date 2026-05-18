package runner

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/pipeline"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Runner is the main application entry point
type SeqRunner struct {
	ctx  context.Context
	cfg  *config.Config
	jobs []config.JobConfig
}

func NewSeqRunner(ctx context.Context, cfg *config.Config) *SeqRunner {
	return &SeqRunner{
		ctx:  ctx,
		cfg:  cfg,
		jobs: make([]config.JobConfig, 0),
	}
}

func (sr *SeqRunner) Run() error {
	stages := sr.cfg.Stages
	if len(sr.jobs) == 0 {
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

	adapter := docker.NewConfigAdapter(sr.cfg)
	executor := docker.NewDockerExecutor(dockerClient, adapter)

	var runErr error
	p := pipeline.NewPipeline(executor, stages, sr.jobs)
	runErr = p.Run(sr.ctx)

	if runErr != nil {
		return runErr
	}
	return nil
}

func (sr *SeqRunner) Cleanup(ctx context.Context) error {
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

func (sr *SeqRunner) PrepareJobConfigs(options RunnerOptions) error {
	log.Println("Preparing jobs...")
	log.Println("Populating jobs with global variables...")
	for i, job := range sr.cfg.Jobs {
		if job.Variables == nil {
			sr.cfg.Jobs[i].Variables = make(map[string]string)
		}
		//local variables take precedence over global variables
		for k, v := range sr.cfg.GlobalVariables {
			if _, ok := job.Variables[k]; !ok {
				sr.cfg.Jobs[i].Variables[k] = v
			}
		}
	}

	// TODO: This feels bad man...

	if len(options.jobNames) != 0 {
		for _, job := range sr.cfg.Jobs {
			if slices.Contains(options.jobNames, job.Name) {
				sr.jobs = append(sr.jobs, job)
			}
		}
		return nil
	}

	if len(options.stages) != 0 {
		for _, s := range options.stages {
			if !slices.Contains(sr.cfg.Stages, s) {
				return fmt.Errorf("Requested stage %q is not present in config file: %q", s, sr.cfg.FileName)
			}
		}
		for _, job := range sr.cfg.Jobs {
			if slices.Contains(options.stages, job.Stage) {
				sr.jobs = append(sr.jobs, job)
			}
		}
		return nil
	}

	for _, s := range sr.cfg.Stages {
		for _, job := range sr.cfg.Jobs {
			if job.Stage == s {
				sr.jobs = append(sr.jobs, job)
			}
		}
	}
	return nil
}
