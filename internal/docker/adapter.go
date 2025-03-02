package docker

import (
	"fmt"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"log"
	"path/filepath"
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
		WorkingDir: job.GetWorkdir(),
		Cmd:        []string{"/bin/sh", "-c", a.buildCmd(job.GetScripts())},
		Env:        a.transformEnvVars(job.GetVariables()),
	}
}

func (a *configAdapter) ToHostConfig(job job.Job) *container.HostConfig {
	return &container.HostConfig{
		Mounts:      a.getMounts(job.GetCache(), job.GetWorkdir(), job.GetName()),
		NetworkMode: a.getNetworkMode(job.GetNetwork()),
		ExtraHosts:  a.getExtraHosts(job.GetNetwork()),
	}
}

func (a *configAdapter) buildCmd(scripts []string) string {
	log.Println("[Docker] Preparing scripts...")
	return strings.Join(scripts, "&&")
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

func (a *configAdapter) getMounts(cache *config.CacheConfig, workdir string, jobName string) []mount.Mount {
	if cache == nil {
		return nil
	}

	var mounts []mount.Mount
	for _, dest := range cache.Paths {
		target := workdir
		// Ensure we have an absolute path
		if !strings.HasSuffix(target, "/") && !strings.HasPrefix(dest, "/") {
			target += "/"
		}
		target += dest

		// Ensure the final path is absolute
		if !filepath.IsAbs(target) {
			target = "/" + target
		}
		// Create a safe volume name
		safePath := strings.ReplaceAll(target, "/", "-")
		sourceName := fmt.Sprintf("%s-%s%s", jobName, cache.Key, safePath)

		log.Printf("[Docker] Creating mount: %s -> %s\n", sourceName, target)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: sourceName,
			Target: target,
		})
	}

	return mounts
}

func (a *configAdapter) getNetworkMode(jobNetwork *config.NetworkConfig) container.NetworkMode {
	if jobNetwork == nil {
		return ""
	}
	log.Println("[Docker] Resolving network mode")
	var networkMode container.NetworkMode
	if jobNetwork.HostAccess {
		networkMode = "host"
	}
	return networkMode
}

func (a *configAdapter) getExtraHosts(jobNetwork *config.NetworkConfig) []string {
	if jobNetwork == nil {
		return nil
	}
	log.Println("[Docker] Resolving extra hosts")
	var extraHosts []string
	if jobNetwork.HostMode {
		extraHosts = append(extraHosts, "host.docker.internal:host-gateway")
	}
	return extraHosts
}
