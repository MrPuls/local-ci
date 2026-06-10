package docker

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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

// CreateContainer creates the job's container. netCfg is non-nil only when the
// job declares services, attaching it to the per-job service network.
func (c *ContainerManager) CreateContainer(ctx context.Context, job config.JobConfig, netCfg *network.NetworkingConfig) (container.CreateResponse, error) {
	c.logger.Println("[Docker] Creating necessary configs...")
	containerCfg := c.adapter.ToContainerConfig(job)
	hostCfg := c.adapter.ToHostConfig(job)
	c.logger.Println("[Docker] Creating container...")
	return c.client.ContainerCreate(ctx, containerCfg, hostCfg, netCfg, nil, job.Name)
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

func (c *ContainerManager) InspectContainer(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return c.client.ContainerInspect(ctx, containerID)
}

// CreateNetwork creates the per-job bridge network services and the job attach
// to. It carries the created_by label so run-level cleanup can sweep leftovers.
func (c *ContainerManager) CreateNetwork(ctx context.Context, name string) (string, error) {
	c.logger.Printf("[Docker] Creating network %q...", name)
	resp, err := c.client.NetworkCreate(ctx, name, network.CreateOptions{
		Labels: map[string]string{"created_by": "local-ci"},
	})
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *ContainerManager) RemoveNetwork(ctx context.Context, networkID string) error {
	c.logger.Printf("[Docker] Removing network %q...", networkID)
	return c.client.NetworkRemove(ctx, networkID)
}

func (c *ContainerManager) ListNetworks(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
	return c.client.NetworkList(ctx, options)
}

// ExecProbe runs `/bin/sh -c cmd` inside a running container and returns its
// exit code. It backs service readiness checks (e.g. pg_isready).
func (c *ContainerManager) ExecProbe(ctx context.Context, containerID, cmd string) (int, error) {
	resp, err := c.client.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd: []string{"/bin/sh", "-c", cmd},
	})
	if err != nil {
		return -1, err
	}
	if err := c.client.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{}); err != nil {
		return -1, err
	}
	for {
		ins, err := c.client.ContainerExecInspect(ctx, resp.ID)
		if err != nil {
			return -1, err
		}
		if !ins.Running {
			return ins.ExitCode, nil
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
