package cli

import (
	"github.com/spf13/cobra"
)

var (
	version = "0.0.11"
)

// rootCmd represents the base command
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

func init() {
	rootCmd.AddCommand(newRunCmd())
}
