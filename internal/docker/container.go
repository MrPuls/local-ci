package docker

import (
	"context"
	"io"
	"log"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerManager struct {
	client  *client.Client
	adapter ConfigAdapter
	logger  *log.Logger
}

func NewContainerManager(cli *client.Client, adapter ConfigAdapter, logger *log.Logger) *ContainerManager {
	return &ContainerManager{
		client:  cli,
		adapter: adapter,
		logger:  logger,
	}
}

func (c *ContainerManager) CreateContainer(ctx context.Context, job config.JobConfig) (container.CreateResponse, error) {
	c.logger.Println("[Docker] Creating necessary configs...")
	containerCfg := c.adapter.ToContainerConfig(job)
	hostCfg := c.adapter.ToHostConfig(job)
	c.logger.Println("[Docker] Creating container...")
	return c.client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, job.Name)
}

func (c *ContainerManager) StartContainer(ctx context.Context, containerID string, options container.StartOptions) error {
	c.logger.Printf("[Docker] Starting container %q...", containerID)
	return c.client.ContainerStart(ctx, containerID, options)
}

func (c *ContainerManager) StopContainer(ctx context.Context, containerID string, options container.StopOptions) error {
	c.logger.Printf("[Docker] Stopping container %q...", containerID)
	return c.client.ContainerStop(ctx, containerID, options)
}

func (c *ContainerManager) RemoveContainer(ctx context.Context, containerID string, options container.RemoveOptions) error {
	c.logger.Printf("[Docker] Removing container %q...", containerID)
	return c.client.ContainerRemove(ctx, containerID, options)
}

func (c *ContainerManager) CopyToContainer(ctx context.Context, containerID string, dest string, content io.Reader, options container.CopyToContainerOptions) error {
	c.logger.Printf("[Docker] Copying files from %q to container %q...", dest, containerID)
	return c.client.CopyToContainer(ctx, containerID, dest, content, options)
}

func (c *ContainerManager) AttachLogger(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
	return c.client.ContainerAttach(ctx, containerID, options)
}

func (c *ContainerManager) WaitForContainer(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return c.client.ContainerWait(ctx, containerID, condition)
}

func (c *ContainerManager) ListContainers(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	c.logger.Printf("[Docker] Listing containers...")
	return c.client.ContainerList(ctx, options)
}
