package cli

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/sink/recorder"
	"github.com/MrPuls/local-ci/internal/sink/terminal"
	"github.com/MrPuls/local-ci/internal/store"
	"github.com/spf13/cobra"
)

var (
	version        = "0.1.3"
	configFile     string
	jobs           []string
	stages         []string
	remote         string
	env            []string
	parallel       bool
	parallelStages bool
	noRecord       bool
)

// openStore opens the run-history store at its default location. Shared by the
// run command (to record) and the runs/log commands (to read).
func openStore() (*store.Store, error) {
	path, err := store.DefaultDBPath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

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
			mode := engine.ModeSequential
			switch {
			case parallelStages:
				mode = engine.ModeParallelStages
			case parallel:
				mode = engine.ModeParallel
			}

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
			defer cancel()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-signals
				log.Println("Operation interrupted, initiating graceful shutdown...")
				log.Println("Stopping runner...")
				cancel()
			}()

			sinks := []engine.Sink{terminal.New(os.Stdout, os.Stderr)}
			if !noRecord {
				if st, err := openStore(); err != nil {
					log.Printf("run history disabled: %v", err)
				} else {
					defer st.Close()
					sinks = append(sinks, recorder.New(st))
				}
			}

			bus := engine.NewBus(sinks...)
			return engine.Run(ctx, engine.Spec{
				ConfigFile: configFile,
				JobNames:   jobs,
				Stages:     stages,
				Remote:     remote,
				Env:        env,
				Mode:       mode,
			}, bus)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configFile, "config", "c", ".local-ci.yaml", "Path to configuration file")
	cmd.Flags().StringSliceVarP(&jobs, "job", "j", []string{}, "Run a specific job(-s) from a configuration file")
	cmd.Flags().StringSliceVarP(&stages, "stage", "s", []string{}, "Run a specific stage(-s) from a configuration file")
	cmd.Flags().StringVarP(&remote, "remote", "r", "", "Pull a remote repo locally and run it's local-ci.yaml file")
	cmd.Flags().StringSliceVarP(&env, "env", "e", []string{}, "Set environment variables for the pipeline")
	cmd.Flags().BoolVarP(&parallel, "parallel", "p", false, "Run all jobs in parallel, ignoring stages")
	cmd.Flags().BoolVar(&parallelStages, "parallel-stages", false, "Run stages in order, with jobs inside each stage in parallel")
	cmd.Flags().BoolVar(&noRecord, "no-record", false, "Do not record this run to the local history database")
	cmd.MarkFlagsMutuallyExclusive("parallel", "parallel-stages")

	return cmd
}

func init() {
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newRunsCmd())
	rootCmd.AddCommand(newLogCmd())
}
