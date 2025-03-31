package app

import (
	"context"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/pipeline"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"log"
)

// Runner is the main application entry point
type Runner struct{}

type RunnerOptions struct {
	jobNames []string
	stages   []string
}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context, configFile *config.Config, options RunnerOptions) error {
	stages := globals.NewStages(configFile)
	variables := globals.NewVariables(configFile)

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

	// TODO:
	//  if jobs - run specific job name,
	// 	if stages = run specific stages,
	//  if both - run specific jobs from specific stages

	// TODO: This ^ looks overly complicated since it basically does the same thing: iterates through jobs with
	//  different filter anb appends them to slice and then runs those jobs. So perhaps the abstraction in needed.
	//  Getting the runner options and filtering jobs

	var runErr error
	jobs := prepareJobs(options)
	p := pipeline.NewPipeline(executor, stages, variables, jobs)
	runErr = p.Run(ctx)

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
