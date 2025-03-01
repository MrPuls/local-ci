package cli

import (
	"github.com/MrPuls/local-ci/app"
	"github.com/spf13/cobra"
)

var (
	configFile string
	job        string
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run pipeline",
		Long:  "Run CI pipeline based on configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := app.NewRunner()
			return runner.Run(configFile, job)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configFile, "config", "c", ".local-ci.yaml", "Path to configuration file")
	cmd.Flags().StringVarP(&job, "job", "j", "", "Run a specific job from a config file")

	return cmd
}
