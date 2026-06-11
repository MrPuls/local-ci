package cli

import (
	"io"
	"log"
	"os"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/spf13/cobra"
)

// Dynamic shell completions. The CLI's arguments are mostly names defined in
// the project's YAML (jobs, stages) or in the run history (run ids), so the
// shell can offer the real values: `local-ci shell <TAB>` lists this config's
// jobs. Every helper is silent (loaders chat on the global logger, which would
// corrupt the completion protocol) and degrades to "no suggestions" on any
// error — completion must never break a command.

// completionConfig loads the config the command would use: the -c flag when
// set, otherwise the first discovered candidate.
func completionConfig(cmd *cobra.Command) *config.Config {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	file, _ := cmd.Flags().GetString("config")
	if file == "" {
		file = config.CanonicalConfigName
	}
	if !cmd.Flags().Changed("config") {
		if candidates, err := config.DiscoverConfigs("."); err == nil && len(candidates) > 0 {
			file = candidates[0]
		}
	}
	cfg := config.NewConfig(file)
	if err := cfg.LoadConfig(); err != nil {
		return nil
	}
	return cfg
}

// completeJobNames suggests job names (post-extends, pre-matrix — the names
// the engine selects on).
func completeJobNames(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg := completionConfig(cmd)
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(cfg.Jobs))
	for _, j := range cfg.Jobs {
		out = append(out, j.Name+"\tstage: "+j.Stage)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeStages(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg := completionConfig(cmd)
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return cfg.Stages, cobra.ShellCompDirectiveNoFileComp
}

// completeConfigFiles suggests the discovered config candidates, falling back
// to regular file completion (the flag accepts any path).
func completeConfigFiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	candidates, err := config.DiscoverConfigs(".")
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return candidates, cobra.ShellCompDirectiveDefault
}

// completeRunIDs suggests recent run ids for this project (status and start
// time ride along as the description column).
func completeRunIDs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	st, err := openStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer st.Close()
	project, _ := os.Getwd()
	runs, err := st.ListRuns(project, false, 20, 0)
	if err != nil || len(runs) == 0 {
		// No runs here: widen to all projects so completion still helps.
		runs, err = st.ListRuns("", true, 20, 0)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	out := make([]string, 0, len(runs))
	for _, r := range runs {
		out = append(out, r.ID+"\t"+r.Status+" · "+fmtTime(r.StartedAt))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// registerCompletions wires the dynamic completion functions onto the
// commands. Called from init after all commands are registered; registration
// errors are impossible in practice (flag names are static) and ignored.
func registerCompletions(run, shell, validate, runs, logCmd *cobra.Command) {
	_ = run.RegisterFlagCompletionFunc("job", completeJobNames)
	_ = run.RegisterFlagCompletionFunc("stage", completeStages)
	_ = run.RegisterFlagCompletionFunc("config", completeConfigFiles)

	shell.ValidArgsFunction = func(cmd *cobra.Command, args []string, tc string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeJobNames(cmd, args, tc)
	}
	_ = shell.RegisterFlagCompletionFunc("config", completeConfigFiles)

	validate.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		candidates, _ := config.DiscoverConfigs(".")
		return candidates, cobra.ShellCompDirectiveDefault
	}
	_ = validate.RegisterFlagCompletionFunc("config", completeConfigFiles)

	runs.ValidArgsFunction = completeRunIDs
	logCmd.ValidArgsFunction = completeRunIDs
	_ = logCmd.RegisterFlagCompletionFunc("job", completeRunJobNames)
}

// completeRunJobNames suggests the job names recorded for the run id already
// on the command line (`local-ci log <run-id> --job <TAB>`).
func completeRunJobNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	st, err := openStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer st.Close()
	jobs, err := st.GetJobs(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := []string{"pipeline\trun-level diagnostics"}
	for _, j := range jobs {
		out = append(out, j.Name+"\t"+j.Status)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
