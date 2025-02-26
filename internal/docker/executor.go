package docker

import (
	"bytes"
	"context"
	"github.com/MrPuls/local-ci/internal/archive"
	"github.com/MrPuls/local-ci/internal/job"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"log"
	"os"
)

type Executor struct {
	client      *client.Client
	adapter     ConfigAdapter
	containerID string
}

func NewDockerExecutor(client *client.Client, adapter ConfigAdapter) *Executor {
	return &Executor{
		client:  client,
		adapter: adapter,
	}
}

func (e *Executor) Execute(ctx context.Context, job job.Job) error {
	cm := NewContainerManager(e.client, e.adapter)
	im := NewImageManager(e.client)
	log.Println("Parsing working directory...")
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		return wdErr
	}

	log.Println("Pulling image...")
	// options could be switched to adapter type if needed more customization
	_, pullErr := im.PullImage(ctx, job.GetImage(), image.PullOptions{})
	if pullErr != nil {
		return pullErr
	}

	log.Println("Start container creation...")
	containerResp, createErr := cm.CreateContainer(ctx, job)
	if createErr != nil {
		return createErr
	}

	e.containerID = containerResp.ID

	var b bytes.Buffer
	log.Println("Trying to create a fs tar...")
	fsErr := archive.CreateFSTar(wd, &b)
	if fsErr != nil {
		return fsErr
	}

	log.Println("Trying to copy files to container...")
	// options could be switched to adapter type if needed more customization
	copyErr := cm.CopyToContainer(ctx, e.containerID, job.GetWorkdir(), &b, container.CopyToContainerOptions{})
	if copyErr != nil {
		return copyErr
	}

	log.Println("Attaching logger to container...")
	logs, logErr := cm.AttachLogger(ctx, e.containerID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
	if logErr != nil {
		return logErr
	}
	defer logs.Close()

	log.Println("Trying to start a container...")
	if startErr := cm.StartContainer(ctx, e.containerID, container.StartOptions{}); startErr != nil {
		return startErr
	}

	_, stdErr := io.Copy(os.Stdout, logs.Reader)
	if stdErr != nil && stdErr != io.EOF {
		return stdErr
	}

	log.Println("Waiting for container to finish...")
	statusCh, errCh := cm.WaitForContainer(ctx, e.containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	}

	log.Println("All done!\nStarting cleanup...")
	return nil
}

func (e *Executor) Cleanup(ctx context.Context) error {
	cm := NewContainerManager(e.client, e.adapter)
	log.Println("Cleaning up container...")
	return cm.RemoveContainer(ctx, e.containerID, container.RemoveOptions{})
}
