package job

import (
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/globals"
)

type Job struct {
	name      string
	config    config.JobConfig
	variables globals.Variables
}

func NewJobConfig(name string, cfg config.JobConfig, globalVariables globals.Variables) Job {
	return Job{
		name:      name,
		config:    cfg,
		variables: globalVariables,
	}
}

func (c *Job) GetName() string      { return c.name }
func (c *Job) GetStage() string     { return c.config.Stage }
func (c *Job) GetImage() string     { return c.config.Image }
func (c *Job) GetWorkdir() string   { return c.config.Workdir }
func (c *Job) GetScripts() []string { return c.config.Script }
func (c *Job) GetVariables() map[string]string {
	// Create a new map to avoid modifying the original config
	vars := make(map[string]string)

	// Copy global variables first
	gv := c.variables.GetGlobalVariables()
	for k, v := range gv {
		vars[k] = v
	}

	// Override with job-specific variables
	for k, v := range c.config.Variables {
		vars[k] = v
	}

	return vars
}
func (c *Job) GetCache() *config.CacheConfig { return c.config.Cache }
