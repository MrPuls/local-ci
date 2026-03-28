package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
)

func RunBootstrap(cfg *config.BootstrapConfig) error {
	if cfg == nil {
		log.Println("No bootstrap config provided, skipping")
		return nil
	}
	if cfg.Timeout == 0 {
		log.Println("No timeout provided, using default of 5 minutes")
		cfg.Timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Minute)
	defer cancel()
	log.Printf("Running bootstrap with timeout %d minutes\n", cfg.Timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("Running bootstrap command: %s\n", cmd)
		if err := command.Run(); err != nil {
			return fmt.Errorf("bootstrap command failed: %s: %w", cmd, err)
		}
	}
	return nil
}

func RunCleanup(cfg *config.CleanupConfig) {
	if cfg == nil {
		log.Println("No cleanup config provided, skipping")
		return
	}
	if cfg.Timeout == 0 {
		log.Println("No timeout provided, using default of 5 minutes")
		cfg.Timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Minute)
	defer cancel()
	log.Printf("Running cleanup with timeout %d minutes\n", cfg.Timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("Running cleanup command: %s\n", cmd)
		if err := command.Run(); err != nil {
			log.Printf("Cleanup command failed: %s: %v\n", cmd, err)
		}
	}
}
