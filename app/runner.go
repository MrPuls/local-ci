package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/docker"
	"github.com/MrPuls/local-ci/internal/integrations/cmd"
	"github.com/MrPuls/local-ci/internal/integrations/fs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type RunMode int

const (
	ModeSequential RunMode = iota
	ModeParallel
	ModeParallelStages
)

type Runner struct {
	ctx  context.Context
	cfg  *config.Config
	jobs []config.JobConfig
	mode RunMode
}

type RunnerOptions struct {
	jobNames       []string
	stages         []string
	env            map[string]string
	parallel       bool
	parallelStages bool
}

func NewRunner(ctx context.Context, cfg *config.Config) *Runner {
	return &Runner{
		ctx:  ctx,
		cfg:  cfg,
		jobs: make([]config.JobConfig, 0),
		mode: ModeSequential,
	}
}

func (r *Runner) Run() error {
	stages := r.cfg.Stages

	log.Printf("Running jobs for stages %v", stages)
	log.Printf("Running jobs %v", r.jobs)

	if len(r.jobs) == 0 {
		return fmt.Errorf("Job list is empty, nothing to run ¯\\_(ツ)_/¯\naborting... ")
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func(dockerClient *client.Client) {
		err := dockerClient.Close()
		if err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}(dockerClient)

	adapter := docker.NewConfigAdapter(r.cfg)
	executor := docker.NewDockerExecutor(dockerClient, adapter)

	switch r.mode {
	case ModeParallelStages:
		return r.runStagesParallel(executor)
	case ModeParallel:
		return r.runParallel(executor)
	default:
		return r.runSequentially(executor)
	}
}

func (r *Runner) runSequentially(executor *docker.Executor) error {
	for _, j := range r.jobs {
		if err := r.runJob(executor, j, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

// setupRunLogging creates the run log directory and diverts diagnostic log
// output to a pipeline log file so the status board has exclusive control of
// the terminal. The returned func restores the previous log output and closes
// the pipeline log file; callers must defer it.
func setupRunLogging() (string, func(), error) {
	logDir, err := fs.MakeRunLogDir()
	if err != nil {
		return "", nil, err
	}

	pipelineLog, err := os.Create(filepath.Join(logDir, "pipeline.log"))
	if err != nil {
		return "", nil, err
	}

	prevLogOut := log.Writer()
	log.SetOutput(pipelineLog)
	restore := func() {
		log.SetOutput(prevLogOut)
		pipelineLog.Close()
	}
	return logDir, restore, nil
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

// runJobsParallel runs the given jobs concurrently, each writing its output to
// its own log file in logDir, reporting progress to board. It waits for every
// job to finish and returns the joined error of all of them.
func (r *Runner) runJobsParallel(executor *docker.Executor, jobs []config.JobConfig, logDir string, board *StatusBoard) error {
	var wg sync.WaitGroup
	errs := make([]error, len(jobs))
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j config.JobConfig) {
			defer wg.Done()

			f, ferr := os.Create(filepath.Join(logDir, j.Name+".log"))
			if ferr != nil {
				errs[i] = fmt.Errorf("job %s: failed to create log file: %w", j.Name, ferr)
				board.Update(j.Name, StateFailed)
				return
			}
			defer f.Close()

			board.Update(j.Name, StateRunning)
			if jobErr := r.runJob(executor, j, f); jobErr != nil {
				errs[i] = jobErr
				board.Update(j.Name, StateFailed)
				return
			}
			board.Update(j.Name, StatePassed)
		}(i, j)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (r *Runner) runParallel(executor *docker.Executor) error {
	logDir, restore, err := setupRunLogging()
	if err != nil {
		return err
	}
	defer restore()

	fmt.Printf("Running %d jobs in parallel, logs in %s\n", len(r.jobs), logDir)

	board := NewStatusBoard(jobNames(r.jobs), os.Stdout)
	board.Start()
	runErr := r.runJobsParallel(executor, r.jobs, logDir, board)
	board.Stop()

	return runErr
}

// runStagesParallel runs jobs stage by stage: stages execute in their declared
// order, jobs within a stage run concurrently. A stage whose jobs report a
// failure stops the pipeline before the next stage starts.
func (r *Runner) runStagesParallel(executor *docker.Executor) error {
	logDir, restore, err := setupRunLogging()
	if err != nil {
		return err
	}
	defer restore()

	fmt.Printf("Running jobs by stage in parallel, logs in %s\n", logDir)

	for _, group := range r.jobsByStage() {
		fmt.Printf("\nStage: %s\n", group.stage)
		board := NewStatusBoard(jobNames(group.jobs), os.Stdout)
		board.Start()
		stageErr := r.runJobsParallel(executor, group.jobs, logDir, board)
		board.Stop()
		if stageErr != nil {
			return stageErr
		}
	}
	return nil
}

func (r *Runner) runJob(executor *docker.Executor, j config.JobConfig, out io.Writer) error {
	if err := cmd.RunJobBootstrap(j.JobBootstrap, j.Variables, out); err != nil {
		return fmt.Errorf("Job %s bootstrap failed: %w", j.Name, err)
	}

	jobErr := executor.Execute(r.ctx, j, out)

	if cleanupErr := cmd.RunJobCleanup(j.JobCleanup, j.Variables, out); cleanupErr != nil {
		return fmt.Errorf("Job %s cleanup failed: %v", j.Name, cleanupErr)
	}

	if jobErr != nil {
		return fmt.Errorf("Job %s failed: %w", j.Name, jobErr)
	}

	return nil
}

func (r *Runner) Cleanup(ctx context.Context) error {
	log.Println("Starting cleanup...")
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func(dockerClient *client.Client) {
		err := dockerClient.Close()
		if err != nil {
			log.Printf("Error closing docker client: %v", err)
		}
	}(dockerClient)

	cm := docker.NewContainerManager(dockerClient, nil)

	containerList, containerListError := cm.ListContainers(
		ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("label", "created_by=local-ci")),
		},
	)
	if containerListError != nil {
		return containerListError
	}

	log.Printf("Containers found: %d", len(containerList))
	if len(containerList) == 0 {
		log.Println("Nothing to cleanup ¯\\_(ツ)_/¯")
		return nil
	}

	for _, availableContainer := range containerList {
		log.Printf("Deleting container: %q, %v", availableContainer.ID, availableContainer.Names[0])
		stpErr := cm.StopContainer(ctx, availableContainer.ID, container.StopOptions{})
		if stpErr != nil {
			return stpErr
		}
		rmErr := cm.RemoveContainer(ctx, availableContainer.ID, container.RemoveOptions{})
		if rmErr != nil {
			return rmErr
		}
	}
	log.Println("All containers removed!")

	return nil
}

func (r *Runner) PrepareJobConfigs(options RunnerOptions) error {
	log.Println("Preparing jobs...")
	switch {
	case options.parallelStages:
		r.mode = ModeParallelStages
	case options.parallel:
		r.mode = ModeParallel
	default:
		r.mode = ModeSequential
	}
	log.Println("Populating jobs with global variables...")
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

	// TODO: This feels bad man...

	if len(options.jobNames) != 0 {
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.jobNames, job.Name) {
				r.jobs = append(r.jobs, job)
			}
		}
		return nil
	}

	if len(options.stages) != 0 {
		for _, s := range options.stages {
			if !slices.Contains(r.cfg.Stages, s) {
				return fmt.Errorf("Requested stage %q is not present in config file: %q", s, r.cfg.FileName)
			}
		}
		for _, job := range r.cfg.Jobs {
			if slices.Contains(options.stages, job.Stage) {
				r.jobs = append(r.jobs, job)
			}
		}
		return nil
	}

	for _, s := range r.cfg.Stages {
		for _, job := range r.cfg.Jobs {
			if job.Stage == s {
				r.jobs = append(r.jobs, job)
			}
		}
	}
	return nil
}
