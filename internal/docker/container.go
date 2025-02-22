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
	return &container.Config{
		Image:      job.GetImage(),
		WorkingDir: job.GetWorkdir(),
	}
}

func (c *ContainerManager) CreateNewContainer() {}
