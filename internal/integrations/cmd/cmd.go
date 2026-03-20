package cmd

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
)

func RunBootstrap(cfg *config.BootstrapConfig) {
	if cfg == nil {
		log.Println("No bootstrap config provided, skipping")
		return
	}
	if cfg.Timeout == 0 {
		log.Println("No timeout provided, using default of 5 minutes")
		cfg.Timeout = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Minute)
	defer cancel()
	log.Printf("Running bootstrap with timeout %d minutes\n", cfg.Timeout)
	for _, cmd := range cfg.Run {
		splittedCmd := strings.Split(cmd, " ")
		command := exec.CommandContext(ctx, splittedCmd[0], splittedCmd[1:]...)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		log.Printf("Running bootstrap command: %s\n", cmd)
		command.Run()
	}
}
