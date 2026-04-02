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

// Currently there are two variations for global cleanup and per-job cleanup.
// May change this in the future to some more elegant approach, but now would like to keep it simple and
// open to possible changes on different levels

func RunGlobalCleanup(cfg *config.CleanupConfig) {
	if cfg == nil {
		log.Println("[Cleanup] No global cleanup config provided, skipping")
		return
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		log.Println("[Cleanup] No global cleanup timeout provided, using default of 5 minutes")
		timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	log.Printf("[Cleanup] Running cleanup with timeout %d minute(s)\n", timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("[Cleanup] Running command: %s\n", cmd)
		if err := command.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("[Cleanup] Cleanup timed out after %d minute(s) on command: %s: %s", timeout, cmd, err)
			}
			log.Printf("[Cleanup] Cleanup command failed: %s: %v\n", cmd, err)
		}
	}
	log.Println("[Cleanup] Global cleanup completed successfully")
}

func RunJobCleanup(cfg *config.JobCleanupConfig) error {
	if cfg == nil {
		log.Println("[Cleanup] No job cleanup config provided, skipping")
		return nil
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		log.Println("[Cleanup] No job cleanup timeout provided, using default of 5 minutes")
		timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	log.Printf("[Cleanup] Running job cleanup with timeout %d minute(s)\n", timeout)
	for _, cmd := range cfg.Run {
		command := exec.CommandContext(ctx, "sh", "-c", cmd)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("[Cleanup] Running command: %s\n", cmd)
		if err := command.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("[Cleanup] Cleanup timed out after %d minute(s) on command: %s: %w", timeout, cmd, err)
			}
			return fmt.Errorf("[Cleanup] Cleanup command failed: %s: %w", cmd, err)
		}
	}
	log.Println("[Cleanup] Job cleanup completed successfully")
	return nil
}
