package docker

import (
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"log"
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
		Cmd:        []string{"/bin/sh", "-c", a.buildCmd(job.GetScripts())},
		Env:        a.transformEnvVars(job.GetVariables()),
	}
}

func (a *configAdapter) ToHostConfig(job job.Job) *container.HostConfig {
	return &container.HostConfig{
		Mounts: a.getMounts(job.GetCache(), job.GetWorkdir()),
	}
}

func (a *configAdapter) buildCmd(scripts []string) string {
	log.Println("[Docker] Preparing scripts...")
	return strings.Join(scripts, "&&")
}

func (a *configAdapter) getWorkdir(workdir string) string {
	var wd string
	if workdir == "" {
		wd = "/"
	} else {
		wd = workdir
	}
	log.Printf("The workdir is: %s\n", wd)
	return wd
}

func (a *configAdapter) transformEnvVars(variables map[string]string) []string {
	if len(variables) == 0 {
		return nil
	}

	var envVars []string
	log.Println("[Docker] Getting environment variables")
	for k, v := range variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}
	return envVars
}

func (a *configAdapter) getMounts(cache *config.CacheConfig, workdir string) []mount.Mount {
	if cache == nil {
		return nil
	}

	var mounts []mount.Mount
	for _, dest := range cache.Paths {
		log.Printf("[Docker] Creating a mount for '%s'\n", dest)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: cache.Key,
			Target: workdir + dest,
		})
	}
	return mounts
}
