package engine

import (
	"context"
	"io"

	"github.com/MrPuls/local-ci/internal/config"
)

// JobExecutor runs a single job to completion, writing all of the job's output
// (image pull progress and container logs) to out. Internal diagnostics are
// emitted through the logger injected into the concrete implementation, not
// through out. Returning a non-nil error means the job failed.
//
// The interface exists so the engine can run without a Docker daemon in tests
// (via a fake) and so other front-ends can reuse the engine unchanged.
type JobExecutor interface {
	Execute(ctx context.Context, job config.JobConfig, out io.Writer) error
}
