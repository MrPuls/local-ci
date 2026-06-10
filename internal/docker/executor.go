package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/MrPuls/local-ci/internal/archive"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Executor struct {
	client    *client.Client
	adapter   ConfigAdapter
	logger    *log.Logger
	artifacts *artifactStore
}

func NewDockerExecutor(client *client.Client, adapter ConfigAdapter, logger *log.Logger) *Executor {
	return &Executor{
		client:    client,
		adapter:   adapter,
		logger:    logger,
		artifacts: &artifactStore{},
	}
}

// Close releases run-scoped resources (the artifact scratch dir). Callers
// should defer it right after constructing the executor.
func (e *Executor) Close() error {
	return e.artifacts.Close()
}

func (e *Executor) Execute(ctx context.Context, job config.JobConfig, out io.Writer) error {
	cm := NewContainerManager(e.client, e.adapter, e.logger)
	im := NewImageManager(e.client, e.adapter, e.logger)
	e.logger.Println("Parsing working directory...")
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		return wdErr
	}

	// Sidecar services: private network + one container per service, gated on
	// readiness, torn down when the job ends however it ends.
	var netCfg *network.NetworkingConfig
	if len(job.Services) > 0 {
		set, svcErr := e.startServices(ctx, cm, im, job, out)
		if set != nil {
			defer set.teardown()
		}
		if svcErr != nil {
			return svcErr
		}
		netCfg = set.jobEndpoint()
	}

	// options could be switched to adapter type if needed more customization
	reader, pullErr := im.PullImage(ctx, job.Image, image.PullOptions{})
	if pullErr != nil {
		return pullErr
	}

	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			e.logger.Printf("[Docker] Error closing image pull reader: %v", err)
		}
	}(reader)

	// Or io.Copy(ioutil.Discard, reader) is we don't want to stream it to out
	_, readerErr := io.Copy(out, reader)
	if readerErr != nil {
		return readerErr
	}

	e.logger.Println("[Docker] Image is pulled...")
	e.logger.Println("[Docker] Start container creation...")
	// A previous attempt (retry) or a crashed run may have left a container
	// with this name behind.
	_ = cm.RemoveContainerForce(ctx, job.Name)
	containerResp, createErr := cm.CreateContainer(ctx, job, netCfg)
	if createErr != nil {
		return createErr
	}

	containerID := containerResp.ID
	// The container is removed when the attempt ends (with a cancel-proof
	// context) so retries can re-create it under the same name; the run-level
	// label sweep stays as a safety net for crashes.
	defer func() {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), teardownTimeout)
		defer cancel()
		if err := cm.RemoveContainerForce(rmCtx, containerID); err != nil {
			e.logger.Printf("[Docker] Error removing container %q: %v", containerID, err)
		}
	}()

	var b bytes.Buffer
	e.logger.Println("[Docker] Trying to create a fs tar...")
	fsErr := archive.CreateFSTar(wd, &b)
	if fsErr != nil {
		return fsErr
	}

	e.logger.Println("[Docker] Trying to copy files to container...")
	// options could be switched to adapter type if needed more customization
	copyErr := cm.CopyToContainer(ctx, containerID, job.Workdir, &b, container.CopyToContainerOptions{})
	if copyErr != nil {
		return copyErr
	}

	// Overlay artifacts collected from earlier jobs onto the workspace.
	if err := e.injectArtifacts(ctx, cm, containerID, job); err != nil {
		return err
	}

	e.logger.Println("[Docker] Attaching logger to container...")
	logs, logErr := cm.AttachLogger(ctx, containerID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
	if logErr != nil {
		return logErr
	}
	defer logs.Close()

	e.logger.Println("[Docker] Trying to start a container...")
	if startErr := cm.StartContainer(ctx, containerID, container.StartOptions{}); startErr != nil {
		return startErr
	}

	// Stream logs concurrently with the wait: a blocking read on the hijacked
	// attach connection can't be interrupted by ctx, so it must not gate the
	// timeout/cancellation path (the deferred logs.Close unblocks it). The
	// demux strips the raw 8-byte multiplex headers from the log.
	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(out, out, logs.Reader)
		copyDone <- copyErr
	}()

	e.logger.Println("[Docker] Waiting for container to finish...")
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

	// The container stopped; drain the remaining log output.
	if stdErr := <-copyDone; stdErr != nil && stdErr != io.EOF {
		return stdErr
	}

	// The job succeeded: collect its declared artifacts for later jobs.
	if err := e.collectArtifacts(ctx, containerID, job); err != nil {
		return err
	}

	e.logger.Println("[Docker] All done!")
	e.logger.Println("[Docker] Starting cleanup...")
	return nil
}

// injectArtifacts copies every artifact collected so far into the container's
// workdir, in collection order (later jobs win on overlapping paths).
func (e *Executor) injectArtifacts(ctx context.Context, cm *ContainerManager, containerID string, job config.JobConfig) error {
	for _, af := range e.artifacts.snapshot() {
		e.logger.Printf("[Docker] Injecting artifacts from job %q...", af.from)
		f, err := os.Open(af.path)
		if err != nil {
			return fmt.Errorf("open artifact from %s: %w", af.from, err)
		}
		err = cm.CopyToContainer(ctx, containerID, job.Workdir, f, container.CopyToContainerOptions{})
		f.Close()
		if err != nil {
			return fmt.Errorf("inject artifact from %s: %w", af.from, err)
		}
	}
	return nil
}

// collectArtifacts copies the job's declared artifact paths out of its
// (stopped) container into the run's artifact store. A declared path that
// doesn't exist fails the job — a missing artifact is a broken contract.
func (e *Executor) collectArtifacts(ctx context.Context, containerID string, job config.JobConfig) error {
	if job.Artifacts == nil {
		return nil
	}
	for _, p := range job.Artifacts.Paths {
		rel := path.Clean(strings.TrimSuffix(p, "/"))
		src := rel
		if !path.IsAbs(src) {
			src = path.Join(job.Workdir, src)
		}
		e.logger.Printf("[Docker] Collecting artifact %q from job %q...", rel, job.Name)
		rc, _, err := e.client.CopyFromContainer(ctx, containerID, src)
		if err != nil {
			return fmt.Errorf("artifact %q not found in job %s: %w", p, job.Name, err)
		}
		collectErr := e.artifacts.collect(job.Name, rc, path.Dir(rel))
		rc.Close()
		if collectErr != nil {
			return fmt.Errorf("store artifact %q from job %s: %w", p, job.Name, collectErr)
		}
	}
	return nil
}
