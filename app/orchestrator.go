package app

import (
	"context"
	"github.com/MrPuls/local-ci/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	runner := NewRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	cfg := config.NewConfig(configFile)
	if configLoadErr := cfg.LoadConfig(); configLoadErr != nil {
		return configLoadErr
	}

	if validatorErr := config.ValidateConfig(cfg); validatorErr != nil {
		return validatorErr
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	runChan := make(chan error, 1)

	go func() {
		runChan <- runner.Run(ctx, cfg, options.JobNames)
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
