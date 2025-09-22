package docker

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

type ConfigAdapter interface {
	ToContainerConfig(job config.JobConfig) *container.Config
	ToHostConfig(job config.JobConfig) *container.HostConfig
}

type configAdapter struct{}

func NewConfigAdapter() ConfigAdapter {
	return &configAdapter{}
}

func (a *configAdapter) ToContainerConfig(job config.JobConfig) *container.Config {
	return &container.Config{
		Image:      job.Image,
		WorkingDir: job.Workdir,
		Cmd:        []string{"/bin/sh", "-c", a.buildCmd(job.Script)},
		Env:        a.transformEnvVars(job.Variables),
		Labels: map[string]string{
			"created_by":            "local-ci",
			"local-ci.job-name":     strings.ToLower(job.Name),
			"local-ci.created-time": time.Now().Format(time.RFC3339),
		},
	}
}

func (a *configAdapter) ToHostConfig(job config.JobConfig) *container.HostConfig {
	return &container.HostConfig{
		Mounts:      a.getMounts(job.Cache, job.Workdir, job.Name),
		NetworkMode: a.getNetworkMode(job.Network),
		ExtraHosts:  a.getExtraHosts(job.Network),
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
		if !strings.HasSuffix(target, "/") && !strings.HasPrefix(dest, "/") {
			target += "/"
		}
		target += dest

		if !filepath.IsAbs(target) {
			target = "/" + target
		}
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
