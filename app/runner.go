package app

import (
	"context"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/globals"
	"github.com/MrPuls/local-ci/internal/pipeline"
	"github.com/docker/docker/client"
	"log"
	"time"
)

// Runner is the main application entry point
type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(configFile string, jobName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// 1. Load configuration
	cfg := config.NewConfig(configFile)
	configLoadErr := cfg.LoadConfig()
	if configLoadErr != nil {
		return configLoadErr
	}

	// 2. Validate configuration
	if validatorErr := config.ValidateConfig(cfg); validatorErr != nil {
		return validatorErr
	}

	// 3. Create globals
	stages := globals.NewStages(cfg)
	variables := globals.NewVariables(cfg)

	// 4. Set up Docker client and executor
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

	// 5. Create and run pipeline
	if jobName != "" {
		p := pipeline.NewJobSpecificPipeline(executor, variables, jobName, cfg)
		return p.Run(ctx)
	} else {
		p := pipeline.NewPipeline(executor, stages, variables, cfg.Jobs)
		return p.Run(ctx)

	}
}
