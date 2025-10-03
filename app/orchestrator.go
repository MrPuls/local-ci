package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
)

type Orchestrator struct{}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

type OrchestratorOptions struct {
	JobNames []string
	Stages   []string
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
	runner := NewRunner(ctx, cfg)
	prepErr := runner.PrepareJobConfigs(
		RunnerOptions{
			jobNames: options.JobNames,
			stages:   options.Stages},
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
