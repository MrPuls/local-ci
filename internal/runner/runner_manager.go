package runner

import (
	"context"
)

// TODO: Take options from orchestrator and create either seq or parallel runner
// then prepare job configs and run jobs. Orchestrator should only know about how to create a runner, pass it the options, and run it.
// For that, runners methods should not be exposed and called internally by the runner manager.
func NewRunner(ctx context.Context, options RunnerOptions) *Runner {
	return nil
}
