package pipeline

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/MrPuls/local-ci/internal/config"
)

type StageSpecificPipeline struct {
	executor   Executor
	config     *config.Config
	stageNames []string
}

func NewStageSpecificPipeline(executor Executor, stageNames []string, config *config.Config) *StageSpecificPipeline {
	return &StageSpecificPipeline{
		executor:   executor,
		config:     config,
		stageNames: stageNames,
	}
}

func (p *StageSpecificPipeline) Run(ctx context.Context) error {
	var jobs []config.JobConfig
	for _, v := range p.config.Jobs {
		if slices.Contains(p.stageNames, v.Stage) {
			log.Printf("Found the job %s for stage %s", v.Name, v.Stage)
			jobs = append(jobs, v)
		}
	}
	if len(jobs) == 0 {
		return fmt.Errorf("No jobs were found for stages: [%v] ", p.stageNames)
	}

	for _, j := range jobs {
		if err := p.executor.Execute(ctx, j); err != nil {
			return fmt.Errorf("job failed: %v", err)
		}
	}
	return nil
}
