package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/MrPuls/local-ci/internal/config"
)

// DAG scheduling: when any job declares `needs:`, jobs run as soon as their
// dependencies pass instead of marching through stages. A job without needs
// keeps stage semantics (it waits for every job in earlier stages), so mixing
// annotated and plain jobs behaves predictably. Jobs downstream of a failure
// are skipped, not run.

func hasNeeds(jobs []config.JobConfig) bool {
	for i := range jobs {
		if len(jobs[i].Needs) > 0 {
			return true
		}
	}
	return false
}

// dagDeps resolves each prepared job's dependency list to prepared job names.
// Explicit needs naming a matrix job expand to all of its variants (variants
// inherited the base's needs at expansion). Needs referencing jobs filtered
// out of this run (-j/-s) are dropped — the user asked for a subset. A job
// without needs depends on every job in earlier stages, except detached
// (`parallel: true`) jobs, which keep their launch-at-start semantics.
func (r *Runner) dagDeps() map[string][]string {
	stageIdx := make(map[string]int, len(r.cfg.Stages))
	for i, s := range r.cfg.Stages {
		stageIdx[s] = i
	}

	// Prepared jobs by base name: a matrix job's variants all answer for it.
	byBase := make(map[string][]string, len(r.jobs))
	for _, j := range r.jobs {
		base := j.MatrixGroup
		if base == "" {
			base = j.Name
		}
		byBase[base] = append(byBase[base], j.Name)
	}

	deps := make(map[string][]string, len(r.jobs))
	for _, j := range r.jobs {
		if len(j.Needs) > 0 {
			var ds []string
			for _, need := range j.Needs {
				targets := byBase[need]
				if len(targets) == 0 {
					r.diagf("Job %s: need %q is not part of this run, ignoring", j.Name, need)
					continue
				}
				ds = append(ds, targets...)
			}
			deps[j.Name] = ds
			continue
		}
		if j.IsParallel() {
			continue // detached: starts at pipeline launch, no implicit deps
		}
		var ds []string
		for _, other := range r.jobs {
			if other.Name != j.Name && stageIdx[other.Stage] < stageIdx[j.Stage] {
				ds = append(ds, other.Name)
			}
		}
		deps[j.Name] = ds
	}
	return deps
}

type dagResult struct {
	err     error
	skipped bool
}

// runDAG runs every prepared job concurrently, each gated on its dependencies
// finishing successfully. The needs graph is validated acyclic at config load,
// so the waits cannot deadlock.
func (r *Runner) runDAG(executor JobExecutor) error {
	r.notice("Running jobs in dependency (DAG) order...\n")
	deps := r.dagDeps()

	results := make(map[string]*dagResult, len(r.jobs))
	done := make(map[string]chan struct{}, len(r.jobs))
	for _, j := range r.jobs {
		results[j.Name] = &dagResult{}
		done[j.Name] = make(chan struct{})
	}

	const groupID = "dag"
	r.bus.Emit(Event{Type: GroupStarted, RunID: r.runID, GroupID: groupID, GroupKind: GroupParallelAll, Order: jobNames(r.jobs)})

	var wg sync.WaitGroup
	for _, j := range r.jobs {
		wg.Add(1)
		go func(j config.JobConfig) {
			defer wg.Done()
			defer close(done[j.Name])
			res := results[j.Name]
			for _, dep := range deps[j.Name] {
				<-done[dep]
				if dr := results[dep]; dr.err != nil || dr.skipped {
					res.skipped = true
					r.notice("Skipping job %s (dependency %s did not pass)\n", j.Name, dep)
					return
				}
			}
			if r.ctx.Err() != nil {
				res.skipped = true
				return
			}
			res.err = r.runJob(executor, j, Concurrent, groupID)
		}(j)
	}
	wg.Wait()
	r.bus.Emit(Event{Type: GroupFinished, RunID: r.runID, GroupID: groupID, GroupKind: GroupParallelAll})

	var errs []error
	skipped := 0
	for _, j := range r.jobs {
		if results[j.Name].err != nil {
			errs = append(errs, results[j.Name].err)
		}
		if results[j.Name].skipped {
			skipped++
		}
	}
	if skipped > 0 && len(errs) > 0 {
		errs = append(errs, fmt.Errorf("%d job(s) skipped because a dependency failed", skipped))
	}
	return errors.Join(errs...)
}
