package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/MrPuls/local-ci/internal/store"
	"github.com/spf13/cobra"
)

var logJob string

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <run-id>",
		Short: "Print the logs of a recorded run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			return showLog(st, args[0])
		},
	}
	cmd.Flags().StringVarP(&logJob, "job", "j", "", "Show only this job's log (use 'pipeline' for run diagnostics)")
	return cmd
}

func showLog(st *store.Store, id string) error {
	if _, err := st.GetRun(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("run %q not found", id)
		}
		return err
	}
	runDir := st.RunDir(id)

	if logJob != "" {
		file := logJob + ".log"
		if logJob == "pipeline" {
			file = "pipeline.log"
		}
		return printFile(filepath.Join(runDir, file))
	}

	jobs, err := st.GetJobs(id)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		fmt.Printf("=== %s (%s) ===\n", j.Name, j.Status)
		path := j.LogPath
		if path == "" {
			path = filepath.Join(runDir, j.Name+".log")
		}
		if err := printFile(path); err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func printFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("(no log)")
			return nil
		}
		return err
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}
