package docker

import (
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerManager struct {
	client *client.Client
}

func NewContainerManager(cli *client.Client) *ContainerManager {
	return &ContainerManager{client: cli}
}

func makeContainerConfig(job job.Job) *container.Config {
	adt := configAdapter{}
	cfg := adt.ToContainerConfig(job)
	return cfg
}

func makeHostConfig(job job.Job) *container.HostConfig {
	adt := configAdapter{}
	cfg := adt.ToHostConfig(job)
	return cfg
}

func (c *ContainerManager) CreateNewContainer() {
	// TODO
}
