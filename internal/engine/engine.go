package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
	"github.com/MrPuls/local-ci/internal/integrations/git"
	"github.com/MrPuls/local-ci/internal/integrations/gitlab"
	"github.com/docker/docker/client"
)

var cleanupTimeout = 30 * time.Second

// newRunID returns a sortable, collision-safe run identifier: a UTC timestamp
// prefix (lexicographically orderable) plus a short random suffix.
func newRunID() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// Run executes a pipeline described by spec, emitting the full event stream to
// bus. It honors ctx for cancellation (callers wire their own signal handling)
// and always runs container cleanup before returning.
func Run(ctx context.Context, spec Spec, bus *Bus) error {
	runID := newRunID()
	diag := newDiagLogger(bus, runID)

	cfg := config.NewConfig(spec.ConfigFile)
	if configLoadErr := cfg.LoadConfig(); configLoadErr != nil {
		return configLoadErr
	}
	diag.Printf("Config file loaded from %s", spec.ConfigFile)

	if validatorErr := config.ValidateConfig(cfg); validatorErr != nil {
		return validatorErr
	}

	if spec.Remote != "" {
		if err := git.SetupLocal(spec.Remote); err != nil {
			return err
		}
	}

	if len(spec.Env) != 0 {
		cfg.CLIVariables = make(map[string]string)
		for _, env := range spec.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				cfg.CLIVariables[key] = value
			}
		}
	}

	if cfg.RemoteProvider != nil {
		opts := gitlab.GitlabOptions{
			Url:       cfg.RemoteProvider.Url,
			Token:     cfg.RemoteProvider.Token,
			ProjectId: cfg.RemoteProvider.ProjectId,
		}
		gtl := gitlab.NewGitLabUtil(&opts)
		vars, err := gtl.GetRemoteVariables()
		if err != nil {
			return fmt.Errorf("failed to fetch remote variables: %w", err)
		}
		// merge remote variables into global variables, global variables take precedence
		maps.Copy(vars, cfg.GlobalVariables)
		cfg.GlobalVariables = vars
	}

	if err := cmd.RunGlobalBootstrap(cfg.Bootstrap, cfg.GlobalVariables); err != nil {
		return err
	}
	defer cmd.RunGlobalCleanup(cfg.Cleanup, cfg.GlobalVariables)

	runner := newRunner(ctx, cfg, bus, runID, spec.Mode, diag)
	if prepErr := runner.PrepareJobConfigs(RunnerOptions{
		jobNames: spec.JobNames,
		stages:   spec.Stages,
	}); prepErr != nil {
		return prepErr
	}

	wd, _ := os.Getwd()
	start := time.Now()
	bus.Emit(Event{
		Type:        RunStarted,
		RunID:       runID,
		Mode:        spec.Mode,
		HasMatrix:   hasMatrixVariants(runner.jobs),
		HasDetached: hasDetached(runner.jobs),
		Order:       jobNames(runner.jobs),
		ConfigPath:  cfg.FileName,
		ProjectPath: wd,
	})

	runErr := runPipeline(cfg, runner, diag)

	if runErr != nil {
		diag.Printf("Runner finished with error: %v", runErr)
	} else {
		diag.Printf("Runner finished successfully")
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cleanupCancel()
	cleanupErr := runner.Cleanup(cleanupCtx)

	bus.Emit(Event{
		Type:     RunFinished,
		RunID:    runID,
		Duration: time.Since(start),
		Err:      errStr(runErr),
	})

	// NOTE: this mirrors the pre-refactor behavior, which returned the cleanup
	// result and discarded runErr (so a failing pipeline still exits 0). See
	// the plan's follow-up note; changing it is a deliberate later decision.
	return cleanupErr
}

// runPipeline builds the Docker-backed executor and runs the prepared jobs. It
// is split out so the Docker client is created and closed around a single call.
func runPipeline(cfg *config.Config, runner *Runner, diag *log.Logger) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dockerClient.Close(); cerr != nil {
			diag.Printf("Error closing docker client: %v", cerr)
		}
	}()

	adapter := docker.NewConfigAdapter(cfg, diag)
	executor := docker.NewDockerExecutor(dockerClient, adapter, diag)
	return runner.Run(executor)
}
