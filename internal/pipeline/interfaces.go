package pipeline

import (
	"context"
	"github.com/MrPuls/local-ci/internal/job"
)

type Executor interface {
	Execute(ctx context.Context, job job.Job) error
	Cleanup(ctx context.Context) error
}
