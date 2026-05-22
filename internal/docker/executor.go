package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/MrPuls/local-ci/internal/archive"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type Executor struct {
	client  *client.Client
	adapter ConfigAdapter
}

func NewDockerExecutor(client *client.Client, adapter ConfigAdapter) *Executor {
	return &Executor{
		client:  client,
		adapter: adapter,
	}
}

func (e *Executor) Execute(ctx context.Context, job config.JobConfig, out io.Writer) error {
	cm := NewContainerManager(e.client, e.adapter)
	im := NewImageManager(e.client, e.adapter)
	log.Println("Parsing working directory...")
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		return wdErr
	}

	// options could be switched to adapter type if needed more customization
	reader, pullErr := im.PullImage(ctx, job.Image, image.PullOptions{})
	if pullErr != nil {
		return pullErr
	}

	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			log.Printf("[Docker] Error closing image pull reader: %v", err)
		}
	}(reader)

	// Or io.Copy(ioutil.Discard, reader) is we don't want to stream it to out
	_, readerErr := io.Copy(out, reader)
	if readerErr != nil {
		return readerErr
	}

	log.Println("[Docker] Image is pulled...")
	log.Println("[Docker] Start container creation...")
	containerResp, createErr := cm.CreateContainer(ctx, job)
	if createErr != nil {
		return createErr
	}

	containerID := containerResp.ID

	var b bytes.Buffer
	log.Println("[Docker] Trying to create a fs tar...")
	fsErr := archive.CreateFSTar(wd, &b)
	if fsErr != nil {
		return fsErr
	}

	log.Println("[Docker] Trying to copy files to container...")
	// options could be switched to adapter type if needed more customization
	copyErr := cm.CopyToContainer(ctx, containerID, job.Workdir, &b, container.CopyToContainerOptions{})
	if copyErr != nil {
		return copyErr
	}

	log.Println("[Docker] Attaching logger to container...")
	logs, logErr := cm.AttachLogger(ctx, containerID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
	if logErr != nil {
		return logErr
	}
	defer logs.Close()

	log.Println("[Docker] Trying to start a container...")
	if startErr := cm.StartContainer(ctx, containerID, container.StartOptions{}); startErr != nil {
		return startErr
	}

	_, stdErr := io.Copy(out, logs.Reader)
	if stdErr != nil && stdErr != io.EOF {
		return stdErr
	}

	log.Println("[Docker] Waiting for container to finish...")
	statusCh, errCh := cm.WaitForContainer(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.Error != nil {
			return fmt.Errorf("container wait failed: %s", status.Error.Message)
		}
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with non-zero status %d", status.StatusCode)
		}
	}

	log.Println("[Docker] All done!")
	log.Println("[Docker] Starting cleanup...")
	return nil
}
