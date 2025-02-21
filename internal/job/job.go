package job

import (
	"context"
	"github.com/MrPuls/local-ci/internal/config"
)

type Executor interface {
	Execute(ctx context.Context, job Job) error
	Cleanup() error
}

type Job interface {
	GetName() string
	GetStage() string
	GetImage() string
	GetWorkdir() string
	GetScripts() []string
	GetVariables() map[string]string
	GetCache() *config.CacheConfig
}

type jobConfig struct {
	name   string
	config config.JobConfig
}

type CacheConfig interface {
	GetKey() string
	GetPaths() []string
}

func NewJobConfig(name string, cfg config.JobConfig) Job {
	return &jobConfig{
		name:   name,
		config: cfg,
	}
}

func (c *jobConfig) GetName() string                 { return c.name }
func (c *jobConfig) GetStage() string                { return c.config.Stage }
func (c *jobConfig) GetImage() string                { return c.config.Image }
func (c *jobConfig) GetWorkdir() string              { return c.config.Workdir }
func (c *jobConfig) GetScripts() []string            { return c.config.Script }
func (c *jobConfig) GetVariables() map[string]string { return c.config.Variables }
func (c *jobConfig) GetCache() *config.CacheConfig   { return c.config.Cache }
