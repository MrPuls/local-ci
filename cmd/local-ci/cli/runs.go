package cli

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/MrPuls/local-ci/internal/store"
	"github.com/spf13/cobra"
)

var (
	runsAll   bool
	runsLimit int
)

func newRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs [run-id]",
		Short: "List recorded runs, or show one run's details",
		Long:  "With no argument, lists recent runs for the current project. With a run id, shows that run's per-job details.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if len(args) == 1 {
				return showRun(st, args[0])
			}
			return listRuns(st)
		},
	}
	cmd.Flags().BoolVarP(&runsAll, "all", "a", false, "Show runs from all projects, not just the current directory")
	cmd.Flags().IntVarP(&runsLimit, "limit", "n", 20, "Maximum number of runs to list")
	return cmd
}

func listRuns(st *store.Store) error {
	project, _ := os.Getwd()
	runs, err := st.ListRuns(project, runsAll, runsLimit, 0)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		if runsAll {
			fmt.Println("No runs recorded yet.")
		} else {
			fmt.Println("No runs recorded for this project (use --all to list every project).")
		}
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tMODE\tGIT\tSTARTED\tDURATION\tJOBS")
	for _, r := range runs {
		jobs, _ := st.GetJobs(r.ID)
		passed := 0
		for _, j := range jobs {
			if j.Status == store.StatusPassed {
				passed++
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d/%d\n",
			r.ID, r.Status, r.Mode, gitRef(r), fmtTime(r.StartedAt), fmtDur(r.Duration), passed, len(jobs))
	}
	return tw.Flush()
}

// gitRef renders a run's git context as "branch@shortsha" ("-" when the run
// happened outside a git repo or predates git capture).
func gitRef(r store.Run) string {
	if r.Commit == "" {
		return "-"
	}
	sha := r.Commit
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if r.Branch == "" {
		return sha
	}
	return r.Branch + "@" + sha
}

func showRun(st *store.Store, id string) error {
	r, err := st.GetRun(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("run %q not found", id)
		}
		return err
	}

	fmt.Printf("Run %s\n", r.ID)
	fmt.Printf("  Project:  %s\n", r.ProjectPath)
	fmt.Printf("  Config:   %s\n", r.ConfigPath)
	fmt.Printf("  Mode:     %s\n", r.Mode)
	if r.Commit != "" {
		fmt.Printf("  Git:      %s\n", gitRef(r))
	}
	fmt.Printf("  Status:   %s\n", r.Status)
	fmt.Printf("  Started:  %s\n", fmtTime(r.StartedAt))
	fmt.Printf("  Duration: %s\n", fmtDur(r.Duration))
	if r.Error != "" {
		fmt.Printf("  Error:    %s\n", r.Error)
	}

	jobs, err := st.GetJobs(id)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}

	fmt.Println()
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "JOB\tSTAGE\tKIND\tSTATUS\tDURATION")
	for _, j := range jobs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", j.Name, j.Stage, j.ExecKind, j.Status, fmtDur(j.Duration))
	}
	return tw.Flush()
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func fmtDur(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.Round(time.Millisecond).String()
}
