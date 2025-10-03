package pipeline

import (
	"context"

	"github.com/MrPuls/local-ci/internal/config"
)

type Executor interface {
	Execute(ctx context.Context, job config.JobConfig) error
	Cleanup(ctx context.Context) error
}
