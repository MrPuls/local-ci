package cli

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/gitlabimport"
	"github.com/spf13/cobra"
)

var (
	importOut   string
	importForce bool
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a pipeline config from another CI system",
	}
	cmd.AddCommand(newImportGitlabCmd())
	return cmd
}

// newImportGitlabCmd converts a .gitlab-ci.yml into a .local-ci.yaml, printing
// notes for every key that has no local equivalent. The result is validated
// before it is reported as usable.
func newImportGitlabCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gitlab [file]",
		Short: "Convert a .gitlab-ci.yml into a local-ci config",
		Long: "Translates a GitLab CI config into .local-ci.yaml: stages, scripts (with " +
			"before_script folded in), images, variables, services, artifacts, needs, retry, " +
			"timeout, cache, extends, and parallel:matrix all carry over. Keys without a local " +
			"equivalent (rules, only/except, environments, ...) are dropped and listed as notes.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := ".gitlab-ci.yml"
			if len(args) == 1 {
				src = args[0]
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			res, err := gitlabimport.Convert(data)
			if err != nil {
				return err
			}

			if importOut == "-" {
				fmt.Print(string(res.YAML))
			} else {
				if _, err := os.Stat(importOut); err == nil && !importForce {
					return fmt.Errorf("%s already exists; use --force to overwrite or -o to pick another path", importOut)
				}
				if err := os.WriteFile(importOut, res.YAML, 0o644); err != nil {
					return err
				}
				fmt.Printf("Wrote %s\n", importOut)
			}

			if len(res.Notes) > 0 {
				fmt.Fprintf(os.Stderr, "\nConversion notes (%d):\n", len(res.Notes))
				for _, n := range res.Notes {
					fmt.Fprintf(os.Stderr, "  - %s\n", n)
				}
			}

			// Tell the user immediately whether the result runs as-is.
			if importOut != "-" {
				log.SetOutput(io.Discard) // loader chatter would bury the verdict
				defer log.SetOutput(os.Stderr)
				cfg := config.NewConfig(importOut)
				cfgLoadErr := cfg.LoadConfig()
				if cfgLoadErr == nil {
					err = config.ValidateConfig(cfg)
				}
				if cfgLoadErr != nil {
					fmt.Fprintf(os.Stderr, "\nWarning: the imported config does not validate yet:\n%v\n", err)
					fmt.Fprintln(os.Stderr, "Fix the TODOs above, then check with `local-ci validate`.")
				} else {
					fmt.Printf("Imported config is valid: %d stage(s), %d job(s). Try `local-ci run`.\n",
						len(cfg.Stages), len(cfg.Jobs))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&importOut, "output", "o", ".local-ci.yaml", "Output path ('-' for stdout)")
	cmd.Flags().BoolVar(&importForce, "force", false, "Overwrite the output file if it exists")
	return cmd
}
