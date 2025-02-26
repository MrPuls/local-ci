package cli

import (
	"github.com/MrPuls/local-ci/app"
	"github.com/spf13/cobra"
)

var (
	configFile string
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run pipeline",
		Long:  "Run CI pipeline based on configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := app.NewRunner()
			return runner.Run(configFile)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configFile, "config", "c", ".local-ci.yaml", "Path to configuration file")

	return cmd
}
