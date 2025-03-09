package docker

import (
	"context"
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"io"
	"log"
)

type ContainerManager struct {
	client  *client.Client
	adapter ConfigAdapter
}

func NewContainerManager(cli *client.Client, adapter ConfigAdapter) *ContainerManager {
	return &ContainerManager{
		client:  cli,
		adapter: adapter,
	}
}

func (c *ContainerManager) CreateContainer(ctx context.Context, job job.Job) (container.CreateResponse, error) {
	log.Println("Creating necessary configs...")
	containerCfg := c.adapter.ToContainerConfig(job)
	hostCfg := c.adapter.ToHostConfig(job)
	log.Print("Creating container...")
	return c.client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, job.GetName())
}

func (c *ContainerManager) StartContainer(ctx context.Context, containerID string, options container.StartOptions) error {
	log.Printf("Starting container %q...", containerID)
	return c.client.ContainerStart(ctx, containerID, options)
}

func (c *ContainerManager) StopContainer(ctx context.Context, containerID string, options container.StopOptions) error {
	log.Printf("Stopping container %q...", containerID)
	return c.client.ContainerStop(ctx, containerID, options)
}

func (c *ContainerManager) RemoveContainer(ctx context.Context, containerID string, options container.RemoveOptions) error {
	log.Printf("Removing container %q...", containerID)
	return c.client.ContainerRemove(ctx, containerID, options)
}

func (c *ContainerManager) CopyToContainer(ctx context.Context, containerID string, dest string, content io.Reader, options container.CopyToContainerOptions) error {
	log.Printf("Copying files from %q to container %q...", dest, containerID)
	return c.client.CopyToContainer(ctx, containerID, dest, content, options)
}

func (c *ContainerManager) AttachLogger(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
	return c.client.ContainerAttach(ctx, containerID, options)
}

func (c *ContainerManager) WaitForContainer(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return c.client.ContainerWait(ctx, containerID, condition)
}

func (c *ContainerManager) ListContainers(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	log.Printf("Listing containers...")
	return c.client.ContainerList(ctx, options)
}
