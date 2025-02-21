package docker

import (
	"github.com/docker/docker/client"
)

type ContainerManager struct {
	client *client.Client
}

func NewContainerManager(cli *client.Client) *ContainerManager {
	return &ContainerManager{client: cli}
}
