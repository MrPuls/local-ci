package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
)

const defaultReadyTimeout = 60 * time.Second

// teardownTimeout bounds service/network removal; it must keep working when
// the job's context is already cancelled or timed out.
const teardownTimeout = 30 * time.Second

var unsafeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// sanitizeName makes s usable as a Docker object name component.
func sanitizeName(s string) string {
	return strings.Trim(unsafeNameRe.ReplaceAllString(s, "-"), "-.")
}

// serviceSet is the running sidecars of one job: a private network plus one
// container per service. It exists from startServices until teardown, which
// the caller must always run (even when startup failed half-way).
type serviceSet struct {
	cm      *ContainerManager
	logger  *log.Logger
	netID   string
	netName string
	started []startedService
	logWG   sync.WaitGroup
}

type startedService struct {
	id    string
	alias string
}

// jobEndpoint is the networking config that attaches the job's container to
// the service network.
func (s *serviceSet) jobEndpoint() *network.NetworkingConfig {
	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			s.netName: {},
		},
	}
}

// teardown stops and removes the service containers and the network. It uses
// a fresh context so cleanup happens even after cancellation or timeout.
func (s *serviceSet) teardown() {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), teardownTimeout)
	defer cancel()
	for _, svc := range s.started {
		s.logger.Printf("[Docker] Stopping service %q...", svc.alias)
		if err := s.cm.RemoveContainerForce(ctx, svc.id); err != nil {
			s.logger.Printf("[Docker] Error removing service %q: %v", svc.alias, err)
		}
	}
	// Wait for the log streamers to drain before the writer goes away.
	s.logWG.Wait()
	if s.netID != "" {
		if err := s.cm.RemoveNetwork(ctx, s.netID); err != nil {
			s.logger.Printf("[Docker] Error removing service network: %v", err)
		}
	}
}

// RemoveContainerForce force-removes a container regardless of state.
func (c *ContainerManager) RemoveContainerForce(ctx context.Context, containerID string) error {
	return c.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// startServices creates the per-job network, then pulls, starts, and
// readiness-gates every declared service. The returned set is non-nil even on
// error so the caller can always defer its teardown.
func (e *Executor) startServices(ctx context.Context, cm *ContainerManager, im *ImageManager, job config.JobConfig, out io.Writer) (*serviceSet, error) {
	set := &serviceSet{cm: cm, logger: e.logger, netName: "local-ci-" + sanitizeName(job.Name)}

	netID, err := cm.CreateNetwork(ctx, set.netName, e.scope)
	if err != nil {
		return set, fmt.Errorf("create service network: %w", err)
	}
	set.netID = netID

	for _, svc := range job.Services {
		alias := svc.EffectiveAlias()
		if err := e.startService(ctx, cm, im, job, svc, alias, set, out); err != nil {
			return set, fmt.Errorf("service %q: %w", alias, err)
		}
	}

	for _, svc := range job.Services {
		alias := svc.EffectiveAlias()
		if err := e.waitServiceReady(ctx, cm, svc, set.byAlias(alias)); err != nil {
			return set, fmt.Errorf("service %q: %w", alias, err)
		}
		e.logger.Printf("[Docker] Service %q is ready", alias)
	}
	return set, nil
}

func (s *serviceSet) byAlias(alias string) string {
	for _, svc := range s.started {
		if svc.alias == alias {
			return svc.id
		}
	}
	return ""
}

func (e *Executor) startService(ctx context.Context, cm *ContainerManager, im *ImageManager, job config.JobConfig, svc config.ServiceConfig, alias string, set *serviceSet, out io.Writer) error {
	reader, err := im.PullImage(ctx, svc.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull %s: %w", svc.Image, err)
	}
	_, copyErr := io.Copy(out, reader)
	reader.Close()
	if copyErr != nil {
		return copyErr
	}

	env := make([]string, 0, len(svc.Variables))
	for k, v := range svc.Variables {
		env = append(env, k+"="+v)
	}
	name := sanitizeName(job.Name) + "-svc-" + alias
	// A previous crashed run may have left a container with this name behind.
	_ = cm.RemoveContainerForce(ctx, name)

	resp, err := cm.client.ContainerCreate(ctx,
		&container.Config{
			Image: svc.Image,
			Env:   env,
			Labels: map[string]string{
				"created_by":          e.scope,
				"local-ci.service":    alias,
				"local-ci.job-name":   strings.ToLower(job.Name),
				"local-ci.created-at": time.Now().Format(time.RFC3339),
			},
		},
		nil,
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				set.netName: {Aliases: []string{alias}},
			},
		},
		nil, name)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	set.started = append(set.started, startedService{id: resp.ID, alias: alias})

	logs, err := cm.AttachLogger(ctx, resp.ID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return fmt.Errorf("attach logs: %w", err)
	}
	if err := cm.StartContainer(ctx, resp.ID, container.StartOptions{}); err != nil {
		logs.Close()
		return fmt.Errorf("start container: %w", err)
	}

	// Stream the service's output into the job log, demuxed and line-prefixed
	// with its alias, until the service stops (teardown closes the stream).
	pw := newPrefixWriter(out, "[svc "+alias+"] ")
	set.logWG.Add(1)
	go func() {
		defer set.logWG.Done()
		defer logs.Close()
		_, _ = stdcopy.StdCopy(pw, pw, logs.Reader)
		pw.Flush()
	}()
	return nil
}

// waitServiceReady blocks until the service is usable: the configured ready
// command exits 0, the image's HEALTHCHECK reports healthy, or — with neither —
// the container is running. A service that exits while waiting is an error.
func (e *Executor) waitServiceReady(ctx context.Context, cm *ContainerManager, svc config.ServiceConfig, containerID string) error {
	timeout := defaultReadyTimeout
	var probe string
	if svc.Ready != nil {
		probe = svc.Ready.Command
		if svc.Ready.Timeout > 0 {
			timeout = svc.Ready.Timeout.Std()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		ins, err := cm.InspectContainer(ctx, containerID)
		if err != nil {
			return err
		}
		state := ins.State
		if state != nil && !state.Running {
			return fmt.Errorf("exited with code %d before becoming ready", state.ExitCode)
		}
		switch {
		case probe != "":
			if code, err := cm.ExecProbe(ctx, containerID, probe); err == nil && code == 0 {
				return nil
			}
		case state != nil && state.Health != nil:
			if state.Health.Status == "healthy" {
				return nil
			}
		default: // no probe, no healthcheck: running is as ready as we can know
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("not ready after %s", timeout)
		case <-time.After(time.Second):
		}
	}
}

// prefixWriter prefixes every line written through it. It buffers partial
// lines across writes so the prefix only ever lands at line starts.
type prefixWriter struct {
	mu     sync.Mutex
	out    io.Writer
	prefix []byte
	buf    bytes.Buffer
}

func newPrefixWriter(out io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{out: out, prefix: []byte(prefix)}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil { // no full line yet: keep the partial for the next write
			w.buf.Write(line)
			break
		}
		if _, werr := w.out.Write(append(append([]byte{}, w.prefix...), line...)); werr != nil {
			return len(p), werr
		}
	}
	return len(p), nil
}

// Flush writes any buffered partial line (used when the stream ends).
func (w *prefixWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		_, _ = w.out.Write(append(append([]byte{}, w.prefix...), append(w.buf.Bytes(), '\n')...))
		w.buf.Reset()
	}
}
