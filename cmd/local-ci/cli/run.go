package cli

import (
	"github.com/MrPuls/local-ci/app"
	"github.com/spf13/cobra"
)

var (
	configFile string
	jobs       []string
	stages     []string
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run pipeline",
		Long:  "Run CI pipeline based on configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			orchestrator := app.NewOrchestrator()
			return orchestrator.Orchestrate(configFile, app.OrchestratorOptions{
				JobNames: jobs,
				Stages:   stages,
			})
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configFile, "config", "c", ".local-ci.yaml", "Path to configuration file")
	cmd.Flags().StringSliceVarP(&jobs, "job", "j", []string{}, "Run a specific job(-s) from a configuration file")

	return cmd
}
