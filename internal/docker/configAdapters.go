package docker

import (
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"strings"
)

type ConfigAdapter interface {
	ToContainerConfig(job job.Job) *container.Config
	ToHostConfig(job job.Job) *container.HostConfig
}

type configAdapter struct{}

func NewConfigAdapter() ConfigAdapter {
	return &configAdapter{}
}

func (a *configAdapter) ToContainerConfig(job job.Job) *container.Config {
	return &container.Config{
		Image:      job.GetImage(),
		WorkingDir: a.getWorkdir(job.GetWorkdir()),
		Cmd:        []string{"/bin/sh", "-c", a.getScripts(job.GetScripts())},
		Env:        a.getVariables(job.GetVariables()),
	}
}

func (a *configAdapter) ToHostConfig(job job.Job) *container.HostConfig {
	return &container.HostConfig{
		Mounts: a.getMounts(job.GetCache(), job.GetWorkdir()),
	}
}

func (a *configAdapter) getScripts(scripts []string) string {
	fmt.Println("[Docker] Preparing scripts...")
	return strings.Join(scripts, "&&")
}

func (a *configAdapter) getWorkdir(workdir string) string {
	var wd string
	if workdir == "" {
		wd = "/"
	} else {
		wd = workdir
	}
	fmt.Printf("The workdir is: %s\n", wd)
	return wd
}

func (a *configAdapter) getVariables(variables map[string]string) []string {
	var envVars []string
	fmt.Println("[Docker] Getting environment variables")
	for k, v := range variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}
	return envVars
}

func (a *configAdapter) getMounts(cache *config.CacheConfig, workdir string) []mount.Mount {
	var mounts []mount.Mount
	for _, dest := range cache.Paths {
		fmt.Printf("[Docker] Creating a mount for '%s'\n", dest)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: cache.Key,
			Target: workdir + dest,
		})
	}
	return mounts
}
