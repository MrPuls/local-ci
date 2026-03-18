package cli

import (
	"github.com/MrPuls/local-ci/app"
	"github.com/spf13/cobra"
)

var (
	version    = "0.0.19"
	configFile string
	jobs       []string
	stages     []string
	remote     string
	env        []string
)

var rootCmd = &cobra.Command{
	Use:   "local-ci",
	Short: "Local CI is a tool for running CI/CD pipelines locally",
	Long:  `A lightweight tool that allows you to run CI/CD pipelines locally using Docker containers.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	rootCmd.Version = version
	return rootCmd.Execute()
}

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
				Remote:   remote,
				Env:      env,
			})
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configFile, "config", "c", ".local-ci.yaml", "Path to configuration file")
	cmd.Flags().StringSliceVarP(&jobs, "job", "j", []string{}, "Run a specific job(-s) from a configuration file")
	cmd.Flags().StringSliceVarP(&stages, "stage", "s", []string{}, "Run a specific stage(-s) from a configuration file")
	cmd.Flags().StringVarP(&remote, "remote", "r", "", "Pull a remote repo locally and run it's local-ci.yaml file")
	cmd.Flags().StringSliceVarP(&env, "env", "e", []string{}, "Set environment variables for the pipeline")

	return cmd
}

func init() {
	rootCmd.AddCommand(newRunCmd())
}
