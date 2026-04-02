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

// Currently there are two variations for global bootstrap and per-job bootstrap.
// May change this in the future to some more elegant approach, but now would like to keep it simple and
// open to possible changes on different levels

func RunGlobalBootstrap(cfg *config.BootstrapConfig) error {
	if cfg == nil {
		log.Println("[Bootstrap] No global bootstrap config provided, skipping")
		return nil
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		log.Println("[Bootstrap] No global bootstrap timeout provided, using default of 5 minutes")
		timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	log.Printf("[Bootstrap] Running global bootstrap with timeout %d minute(s)\n", timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("[Bootstrap] Running command: %s\n", cmd)
		if err := command.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("[Bootstrap] Bootstrap timed out after %d minute(s) on command: %s: %w", timeout, cmd, err)
			}
			return fmt.Errorf("[Bootstrap] Bootstrap command failed: %s: %w", cmd, err)
		}
	}
	log.Println("[Bootstrap] Global bootstrap completed successfully")
	return nil
}

func RunJobBootstrap(cfg *config.JobBootstrapConfig) error {
	if cfg == nil {
		log.Println("[Bootstrap] No job bootstrap config provided, skipping")
		return nil
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		log.Println("[Bootstrap] No job bootstrap timeout provided, using default of 5 minutes")
		timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	log.Printf("[Bootstrap] Running job bootstrap with timeout %d minute(s)\n", timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("[Bootstrap] Running command: %s\n", cmd)
		if err := command.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("[Bootstrap] Bootstrap timed out after %d minute(s) on command: %s: %w", timeout, cmd, err)
			}
			return fmt.Errorf("[Bootstrap] Bootstrap command failed: %s: %w", cmd, err)
		}
	}
	log.Println("[Bootstrap] Job bootstrap completed successfully")
	return nil
}
