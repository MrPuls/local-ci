package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Runner schedules and executes the prepared jobs, emitting events to the bus.
// It owns no presentation: stdout, log files, and the status board live in the
// sinks attached to the bus.
type Runner struct {
	ctx   context.Context
	cfg   *config.Config
	jobs  []config.JobConfig
	mode  RunMode
	bus   *Bus
	runID string
	diag  *log.Logger
}

func newRunner(ctx context.Context, cfg *config.Config, bus *Bus, runID string, mode RunMode, diag *log.Logger) *Runner {
	return &Runner{
		ctx:   ctx,
		cfg:   cfg,
		jobs:  make([]config.JobConfig, 0),
		mode:  mode,
		bus:   bus,
		runID: runID,
		diag:  diag,
	}
}

type RunnerOptions struct {
	jobNames []string
	stages   []string
}

// diagf emits a diagnostic line. It is nil-safe so the planning helpers remain
// callable from tests that construct a bare Runner.
func (r *Runner) diagf(format string, args ...any) {
	if r.diag != nil {
		r.diag.Printf(format, args...)
	}
}

// notice emits a run-level stdout line (chrome that is not the output of any
// single job, e.g. "Waiting for detached jobs to finish..."). Modeled as a
// Standalone LogLine so the terminal sink streams it to stdout.
func (r *Runner) notice(format string, args ...any) {
	r.bus.Emit(Event{
		Type:   LogLine,
		RunID:  r.runID,
		Exec:   Standalone,
		Stream: StreamStdout,
		Data:   []byte(fmt.Sprintf(format, args...)),
	})
}

func (r *Runner) Run(executor JobExecutor) error {
	stages := r.cfg.Stages

	r.diagf("Running jobs for stages %v", stages)
	r.diagf("Running jobs %v", r.jobs)

	if len(r.jobs) == 0 {
		return fmt.Errorf("Job list is empty, nothing to run ¯\\_(ツ)_/¯\naborting... ")
	}

	// 'needs' supersedes the run mode: dependency order subsumes both the
	// sequential chain and the stage barriers.
	if hasNeeds(r.jobs) {
		if r.mode != ModeSequential {
			r.diagf("Config uses 'needs'; run mode is superseded by dependency (DAG) order")
		}
		return r.runDAG(executor)
	}

	switch r.mode {
	case ModeParallelStages:
		return r.runStagesParallel(executor)
	case ModeParallel:
		return r.runParallel(executor)
	default:
		if hasDetached(r.jobs) {
			return r.runSequentialWithDetached(executor)
		}
		return r.runSequentially(executor)
	}
}

func (r *Runner) runSequentially(executor JobExecutor) error {
	return r.iterateSequential(executor, r.jobs)
}

// iterateSequential walks jobs in order, emitting each non-matrix job as a
// Standalone job and coalescing consecutive jobs that share a MatrixGroup into
// a concurrent barrier.
func (r *Runner) iterateSequential(executor JobExecutor, jobs []config.JobConfig) error {
	i := 0
	for i < len(jobs) {
		j := jobs[i]
		if j.MatrixGroup != "" {
			end := findMatrixGroupEnd(jobs, i)
			if err := r.runMatrixBarrier(executor, jobs[i:end]); err != nil {
				return err
			}
			i = end
			continue
		}
		if err := r.runJob(executor, j, Standalone, ""); err != nil {
			return err
		}
		i++
	}
	return nil
}

// runMatrixBarrier runs a contiguous group of matrix variants concurrently,
// bracketed by GroupStarted/GroupFinished so the sink can present a board.
func (r *Runner) runMatrixBarrier(executor JobExecutor, group []config.JobConfig) error {
	label := group[0].MatrixGroup
	groupID := "matrix:" + label
	r.bus.Emit(Event{Type: GroupStarted, RunID: r.runID, GroupID: groupID, GroupKind: GroupMatrix, GroupLabel: label, Order: jobNames(group)})
	runErr := r.runJobsParallel(executor, group, groupID)
	r.bus.Emit(Event{Type: GroupFinished, RunID: r.runID, GroupID: groupID, GroupKind: GroupMatrix, GroupLabel: label})
	return runErr
}

// findMatrixGroupEnd returns end such that jobs[start:end] are all the
// consecutive jobs sharing jobs[start].MatrixGroup. Assumes jobs[start] has
// a non-empty MatrixGroup.
func findMatrixGroupEnd(jobs []config.JobConfig, start int) int {
	group := jobs[start].MatrixGroup
	end := start + 1
	for end < len(jobs) && jobs[end].MatrixGroup == group {
		end++
	}
	return end
}

func hasMatrixVariants(jobs []config.JobConfig) bool {
	for _, j := range jobs {
		if j.MatrixGroup != "" {
			return true
		}
	}
	return false
}

func hasDetached(jobs []config.JobConfig) bool {
	for i := range jobs {
		if jobs[i].IsParallel() {
			return true
		}
	}
	return false
}

func partitionByParallel(jobs []config.JobConfig) (sequential, detached []config.JobConfig) {
	for _, j := range jobs {
		if j.IsParallel() {
			detached = append(detached, j)
		} else {
			sequential = append(sequential, j)
		}
	}
	return
}

// runSequentialWithDetached runs jobs marked `parallel: true` as detached
// goroutines starting at pipeline launch, while remaining jobs run
// sequentially. The sequential chain stops on first failure but the pipeline
// waits for all detached jobs to finish before returning the aggregate error.
func (r *Runner) runSequentialWithDetached(executor JobExecutor) error {
	sequential, detached := partitionByParallel(r.jobs)

	var wg sync.WaitGroup
	detachedErrs := make([]error, len(detached))
	for i, j := range detached {
		wg.Add(1)
		go func(i int, j config.JobConfig) {
			defer wg.Done()
			detachedErrs[i] = r.runJob(executor, j, Detached, "")
		}(i, j)
	}

	seqErr := r.iterateSequential(executor, sequential)

	if len(detached) > 0 {
		r.notice("Waiting for detached jobs to finish...\n")
	}
	wg.Wait()

	return errors.Join(append([]error{seqErr}, detachedErrs...)...)
}

type stageJobs struct {
	stage string
	jobs  []config.JobConfig
}

// jobsByStage groups the prepared jobs by stage, preserving the stage order
// declared in the config. Stages with no jobs are omitted.
func (r *Runner) jobsByStage() []stageJobs {
	var groups []stageJobs
	for _, stage := range r.cfg.Stages {
		var jobs []config.JobConfig
		for _, j := range r.jobs {
			if j.Stage == stage {
				jobs = append(jobs, j)
			}
		}
		if len(jobs) > 0 {
			groups = append(groups, stageJobs{stage: stage, jobs: jobs})
		}
	}
	return groups
}

func jobNames(jobs []config.JobConfig) []string {
	names := make([]string, len(jobs))
	for i, j := range jobs {
		names[i] = j.Name
	}
	return names
}

// runJobsParallel runs the given jobs concurrently and waits for every job to
// finish, returning the joined error. Per-job presentation (board state, log
// files) is handled by sinks reacting to the JobStarted/JobFinished events
// runJob emits.
func (r *Runner) runJobsParallel(executor JobExecutor, jobs []config.JobConfig, groupID string) error {
	var wg sync.WaitGroup
	errs := make([]error, len(jobs))
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j config.JobConfig) {
			defer wg.Done()
			errs[i] = r.runJob(executor, j, Concurrent, groupID)
		}(i, j)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (r *Runner) runParallel(executor JobExecutor) error {
	groupID := "parallel"
	r.bus.Emit(Event{Type: GroupStarted, RunID: r.runID, GroupID: groupID, GroupKind: GroupParallelAll, Order: jobNames(r.jobs)})
	runErr := r.runJobsParallel(executor, r.jobs, groupID)
	r.bus.Emit(Event{Type: GroupFinished, RunID: r.runID, GroupID: groupID, GroupKind: GroupParallelAll})
	return runErr
}

// runStagesParallel runs jobs stage by stage: stages execute in their declared
// order, jobs within a stage run concurrently. A stage whose jobs report a
// failure stops the pipeline before the next stage starts.
func (r *Runner) runStagesParallel(executor JobExecutor) error {
	for _, group := range r.jobsByStage() {
		groupID := "stage:" + group.stage
		r.bus.Emit(Event{Type: GroupStarted, RunID: r.runID, GroupID: groupID, GroupKind: GroupStage, GroupLabel: group.stage, Order: jobNames(group.jobs)})
		stageErr := r.runJobsParallel(executor, group.jobs, groupID)
		r.bus.Emit(Event{Type: GroupFinished, RunID: r.runID, GroupID: groupID, GroupKind: GroupStage, GroupLabel: group.stage})
		if stageErr != nil {
			return stageErr
		}
	}
	return nil
}

func (r *Runner) runJob(executor JobExecutor, j config.JobConfig, exec ExecKind, groupID string) error {
	start := time.Now()
	r.bus.Emit(Event{Type: JobStarted, RunID: r.runID, Job: j.Name, Stage: j.Stage, Exec: exec, GroupID: groupID})

	out := &logWriter{bus: r.bus, runID: r.runID, job: j.Name, stage: j.Stage, exec: exec, groupID: groupID}
	err := r.runJobInner(executor, j, out)

	r.bus.Emit(Event{
		Type:     JobFinished,
		RunID:    r.runID,
		Job:      j.Name,
		Stage:    j.Stage,
		Exec:     exec,
		GroupID:  groupID,
		Duration: time.Since(start),
		ExitCode: exitCode(err),
		Err:      errStr(err),
	})
	return err
}

// runJobInner runs a job's attempts: up to 1+retry tries, each under the
// job's own timeout when one is set. A cancelled run is never retried.
func (r *Runner) runJobInner(executor JobExecutor, j config.JobConfig, out io.Writer) error {
	attempts := j.Retry + 1
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			r.notice("Job %s failed, retrying (attempt %d/%d)...\n", j.Name, attempt, attempts)
		}
		err = r.runJobAttempt(executor, j, out)
		if err == nil || r.ctx.Err() != nil {
			break
		}
	}
	return err
}

func (r *Runner) runJobAttempt(executor JobExecutor, j config.JobConfig, out io.Writer) error {
	ctx := r.ctx
	if j.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, j.Timeout.Std())
		defer cancel()
	}

	if err := cmd.RunJobBootstrap(j.JobBootstrap, j.Variables, out, r.diag); err != nil {
		return fmt.Errorf("Job %s bootstrap failed: %w", j.Name, err)
	}

	jobErr := executor.Execute(ctx, j, out)
	// Distinguish the job's own deadline from a run-wide cancellation.
	if jobErr != nil && ctx.Err() == context.DeadlineExceeded && r.ctx.Err() == nil {
		jobErr = fmt.Errorf("timed out after %s: %w", j.Timeout.Std(), jobErr)
	}

	if cleanupErr := cmd.RunJobCleanup(j.JobCleanup, j.Variables, out, r.diag); cleanupErr != nil {
		return fmt.Errorf("Job %s cleanup failed: %v", j.Name, cleanupErr)
	}

	if jobErr != nil {
		return fmt.Errorf("Job %s failed: %w", j.Name, jobErr)
	}

	return nil
}

func (r *Runner) Cleanup(ctx context.Context) error {
	r.diagf("Starting cleanup...")
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func(dockerClient *client.Client) {
		if cerr := dockerClient.Close(); cerr != nil {
			r.diagf("Error closing docker client: %v", cerr)
		}
	}(dockerClient)

	cm := docker.NewContainerManager(dockerClient, nil, r.diag)

	containerList, containerListError := cm.ListContainers(
		ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("label", "created_by=local-ci")),
		},
	)
	if containerListError != nil {
		return containerListError
	}

	r.diagf("Containers found: %d", len(containerList))
	if len(containerList) == 0 {
		r.diagf("Nothing to cleanup ¯\\_(ツ)_/¯")
		return nil
	}

	for _, availableContainer := range containerList {
		r.diagf("Deleting container: %q, %v", availableContainer.ID, availableContainer.Names[0])
		stpErr := cm.StopContainer(ctx, availableContainer.ID, container.StopOptions{})
		if stpErr != nil {
			return stpErr
		}
		rmErr := cm.RemoveContainer(ctx, availableContainer.ID, container.RemoveOptions{})
		if rmErr != nil {
			return rmErr
		}
	}
	r.diagf("All containers removed!")

	// Sweep leftover service networks (normally removed with their job; this
	// catches crashes and cancellations mid-teardown).
	networks, netListErr := cm.ListNetworks(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "created_by=local-ci")),
	})
	if netListErr != nil {
		return netListErr
	}
	for _, n := range networks {
		r.diagf("Deleting network: %q, %v", n.ID, n.Name)
		if rmErr := cm.RemoveNetwork(ctx, n.ID); rmErr != nil {
			return rmErr
		}
	}

	return nil
}

func (r *Runner) PrepareJobConfigs(options RunnerOptions) error {
	r.diagf("Preparing jobs...")
	r.diagf("Populating jobs with global variables...")
	for i, job := range r.cfg.Jobs {
		if job.Variables == nil {
			r.cfg.Jobs[i].Variables = make(map[string]string)
		}
		//local variables take precedence over global variables
		for k, v := range r.cfg.GlobalVariables {
			if _, ok := job.Variables[k]; !ok {
				r.cfg.Jobs[i].Variables[k] = v
			}
		}
	}

	var filtered []config.JobConfig
	switch {
	case len(options.jobNames) != 0:
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.jobNames, job.Name) {
				filtered = append(filtered, job)
			}
		}
	case len(options.stages) != 0:
		for _, s := range options.stages {
			if !slices.Contains(r.cfg.Stages, s) {
				return fmt.Errorf("Requested stage %q is not present in config file: %q", s, r.cfg.FileName)
			}
		}
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.stages, job.Stage) {
				filtered = append(filtered, job)
			}
		}
	default:
		for _, s := range r.cfg.Stages {
			for _, job := range r.cfg.Jobs {
				if job.Stage == s {
					filtered = append(filtered, job)
				}
			}
		}
	}

	for _, job := range filtered {
		variants, err := config.ExpandMatrix(job)
		if err != nil {
			return err
		}
		r.jobs = append(r.jobs, variants...)
	}
	return nil
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	return 1
}
