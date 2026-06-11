package cli

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

func newDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

var (
	shellConfig  string
	shellVerbose bool
)

// newShellCmd opens an interactive shell inside a job's exact environment —
// the killer debugging move a local CI tool can offer that hosted CI can't.
func newShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell <job>",
		Short: "Open an interactive shell in a job's environment",
		Long: "Starts the job's container — same image, variables, workdir, cache mounts, and " +
			"copied workspace, with its services running on the job network — and attaches an " +
			"interactive /bin/sh. Debug a failing job from the inside instead of guessing.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.NewConfig(shellConfig)
			if err := cfg.LoadConfig(); err != nil {
				return err
			}
			if err := config.ValidateConfig(cfg); err != nil {
				return err
			}

			job, err := findJob(cfg, args[0])
			if err != nil {
				return err
			}
			// Same variable layering a run applies: job vars win over globals.
			if job.Variables == nil {
				job.Variables = make(map[string]string)
			}
			for k, v := range cfg.GlobalVariables {
				if _, ok := job.Variables[k]; !ok {
					job.Variables[k] = v
				}
			}
			if len(job.Matrix) > 0 {
				fmt.Println("Note: this job fans out via matrix; entering the base environment (matrix variables unset).")
			}

			// Docker-layer diagnostics are noise in an interactive session
			// unless asked for.
			diag := log.New(io.Discard, "", 0)
			if shellVerbose {
				diag = log.Default()
			}

			dockerClient, err := newDockerClient()
			if err != nil {
				return err
			}
			defer dockerClient.Close()

			return docker.RunShell(cmd.Context(), dockerClient, cfg, *job, diag)
		},
	}
	cmd.Flags().StringVarP(&shellConfig, "config", "c", ".local-ci.yaml", "Path to configuration file")
	cmd.Flags().BoolVarP(&shellVerbose, "verbose", "v", false, "Show Docker-layer diagnostics")
	return cmd
}

// findJob returns the named job from the loaded config, with a helpful list
// of valid names when there is no match.
func findJob(cfg *config.Config, name string) (*config.JobConfig, error) {
	names := make([]string, 0, len(cfg.Jobs))
	for i := range cfg.Jobs {
		if cfg.Jobs[i].Name == name {
			return &cfg.Jobs[i], nil
		}
		names = append(names, cfg.Jobs[i].Name)
	}
	return nil, fmt.Errorf("job %q not found in %s. Available jobs: %s",
		name, cfg.FileName, strings.Join(names, ", "))
}
