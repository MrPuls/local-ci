package cli

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/spf13/cobra"
)

var validateConfig string

// newValidateCmd lints a config without running anything: load (including
// includes/extends/matrix expansion checks) plus full validation. Exit code 1
// on an invalid config makes it usable in git hooks. Never interactive.
func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a config file without running it",
		Long: "Loads and validates a pipeline config (stages, jobs, includes, templates, " +
			"matrix, needs) and reports the result. Exits non-zero when the config is invalid, " +
			"so it can guard commits and CI.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The config loader chats on the global logger ("Collecting
			// jobs..."); a lint command's output should be just the verdict.
			log.SetOutput(io.Discard)
			defer log.SetOutput(os.Stderr)

			file := validateConfig
			if len(args) == 1 {
				file = args[0]
			}
			cfg := config.NewConfig(file)
			if err := cfg.LoadConfig(); err != nil {
				return fmt.Errorf("%s is INVALID:\n%w", file, err)
			}
			if err := config.ValidateConfig(cfg); err != nil {
				return fmt.Errorf("%s is INVALID:\n%w", file, err)
			}
			variants := 0
			for _, j := range cfg.Jobs {
				v, err := config.ExpandMatrix(j)
				if err != nil {
					return fmt.Errorf("%s is INVALID:\n%w", file, err)
				}
				variants += len(v)
			}
			fmt.Printf("%s is valid: %d stage(s), %d job(s)", cfg.FileName, len(cfg.Stages), len(cfg.Jobs))
			if variants != len(cfg.Jobs) {
				fmt.Printf(" (%d after matrix expansion)", variants)
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&validateConfig, "config", "c", ".local-ci.yaml", "Path to configuration file")
	return cmd
}
