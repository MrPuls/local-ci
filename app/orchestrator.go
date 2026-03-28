package app

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
	"github.com/MrPuls/local-ci/internal/integrations/git"
	"github.com/MrPuls/local-ci/internal/integrations/gitlab"
)

type Orchestrator struct{}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

type OrchestratorOptions struct {
	JobNames []string
	Stages   []string
	Remote   string
	Env      []string
}

var (
	cleanupTimeout = 30 * time.Second
)

func (o *Orchestrator) Orchestrate(configFile string, options OrchestratorOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	cfg := config.NewConfig(configFile)
	if configLoadErr := cfg.LoadConfig(); configLoadErr != nil {
		return configLoadErr
	}
	log.Printf("Config file loaded from %s", configFile)

	if validatorErr := config.ValidateConfig(cfg); validatorErr != nil {
		return validatorErr
	}

	if options.Remote != "" {
		err := git.SetupLocal(options.Remote)
		if err != nil {
			return err
		}
	}

	if len(options.Env) != 0 {
		cfg.CLIVariables = make(map[string]string)
		for _, env := range options.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				cfg.CLIVariables[key] = value
			}
		}
	}

	if cfg.RemoteProvider != nil {
		options := gitlab.GitlabOptions{
			Url:       cfg.RemoteProvider.Url,
			Token:     cfg.RemoteProvider.Token,
			ProjectId: cfg.RemoteProvider.ProjectId,
		}
		gtl := gitlab.NewGitLabUtil(&options)
		vars, err := gtl.GetRemoteVariables()
		if err != nil {
			return fmt.Errorf("failed to fetch remote variables: %w", err)
		}
		// merge remote variables into global variables, global variables take precedence
		maps.Copy(vars, cfg.GlobalVariables)
		cfg.GlobalVariables = vars
	}

	if err := cmd.RunBootstrap(cfg.Bootstrap); err != nil {
		return err
	}
	defer cmd.RunCleanup(cfg.Cleanup)

	runner := NewRunner(ctx, cfg)
	prepErr := runner.PrepareJobConfigs(
		RunnerOptions{
			jobNames: options.JobNames,
			stages:   options.Stages,
		},
	)

	if prepErr != nil {
		return prepErr
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	runChan := make(chan error, 1)

	go func() {
		runChan <- runner.Run()
	}()

	select {
	case err := <-runChan:
		if err != nil {
			log.Printf("Runner finished with error: %v", err)
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cleanupCancel()
			return runner.Cleanup(cleanupCtx)
		} else {
			log.Printf("Runner finished successfully")
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cleanupCancel()
			return runner.Cleanup(cleanupCtx)
		}
	case <-signals:
		log.Println("Operation interrupted, initiating graceful shutdown...")
		cancel()
		log.Println("Stopping runner...")
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cleanupCancel()
		return runner.Cleanup(cleanupCtx)
	}
}
