package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/sink/recorder"
	"github.com/MrPuls/local-ci/internal/sink/terminal"
	"github.com/MrPuls/local-ci/internal/store"
	"github.com/spf13/cobra"
)

var (
	version        = "0.1.4"
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
			cfgFile := resolveConfigFile(cmd)
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
			return engine.Run(ctx, engine.NewRunID(), engine.Spec{
				ConfigFile: cfgFile,
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

// resolveConfigFile decides which config `run` uses when the user didn't pass
// -c: discover the candidates in the working directory and, on a TTY, ask
// which one to load — even a single candidate is confirmed, so the user always
// sees which file is about to run. Non-interactive sessions never block: a
// single candidate is used as-is, anything else keeps the flag default. An
// explicit -c or --remote skips discovery entirely.
func resolveConfigFile(cmd *cobra.Command) string {
	if cmd.Flags().Changed("config") || remote != "" {
		return configFile
	}
	candidates, err := config.DiscoverConfigs(".")
	if err != nil || len(candidates) == 0 {
		return configFile
	}
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		if len(candidates) == 1 {
			return candidates[0]
		}
		return configFile
	}
	return promptConfigChoice(candidates)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func promptConfigChoice(candidates []string) string {
	plural := "s"
	if len(candidates) == 1 {
		plural = ""
	}
	fmt.Printf("%d config file%s found:\n", len(candidates), plural)
	for i, n := range candidates {
		fmt.Printf("  [%d] %s\n", i+1, n)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Which one do you want to load? [1-%d, Enter = 1]: ", len(candidates))
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return candidates[0]
		}
		if n, convErr := strconv.Atoi(line); convErr == nil && n >= 1 && n <= len(candidates) {
			return candidates[n-1]
		}
		if err != nil { // EOF/read error with unusable input: take the default
			return candidates[0]
		}
		fmt.Println("Invalid selection.")
	}
}

func init() {
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newRunsCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newUICmd())
}
