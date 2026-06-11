package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/MrPuls/local-ci/internal/archive"
	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/moby/term"
)

// shellScope is the created_by label for shell sessions: distinct from
// pipeline runs so a concurrent run's end-of-run sweep can't tear down an
// open shell (the session removes its own containers/network on exit).
const shellScope = "local-ci-shell"

// RunShell drops the user into an interactive /bin/sh inside the job's exact
// environment: same image, variables, workdir, cache mounts, copied workspace,
// and — when the job declares services — the same sidecars on the same
// network. Status chatter goes to stderr; stdout stays clean for the TTY.
func RunShell(ctx context.Context, cli *client.Client, cfg *config.Config, job config.JobConfig, logger *log.Logger) error {
	e := NewDockerExecutor(cli, NewConfigAdapter(cfg, logger), logger)
	e.scope = shellScope
	cm := NewContainerManager(cli, e.adapter, logger)
	im := NewImageManager(cli, e.adapter, logger)

	stdinFd, stdinIsTerm := term.GetFdInfo(os.Stdin)
	_, stdoutIsTerm := term.GetFdInfo(os.Stdout)
	if !stdinIsTerm || !stdoutIsTerm {
		return fmt.Errorf("shell requires an interactive terminal")
	}

	status := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	status("Pulling %s...", job.Image)
	reader, err := im.PullImage(ctx, job.Image, image.PullOptions{})
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, reader)
	reader.Close()

	var netCfg *network.NetworkingConfig
	if len(job.Services) > 0 {
		status("Starting %d service(s)...", len(job.Services))
		set, svcErr := e.startServices(ctx, cm, im, job, io.Discard)
		if set != nil {
			defer set.teardown()
		}
		if svcErr != nil {
			return svcErr
		}
		netCfg = set.jobEndpoint()
		for _, svc := range job.Services {
			status("  service %q ready", svc.EffectiveAlias())
		}
	}

	// The job's container config, reshaped for an interactive session: a shell
	// instead of the script, with a TTY and open stdin.
	containerCfg := e.adapter.ToContainerConfig(job)
	containerCfg.Cmd = []string{"/bin/sh"}
	containerCfg.Tty = true
	containerCfg.OpenStdin = true
	containerCfg.AttachStdin = true
	containerCfg.AttachStdout = true
	containerCfg.AttachStderr = true
	containerCfg.Labels["created_by"] = shellScope

	name := sanitizeName(job.Name) + "-shell"
	_ = cm.RemoveContainerForce(ctx, name)
	resp, err := cli.ContainerCreate(ctx, containerCfg, e.adapter.ToHostConfig(job), netCfg, nil, name)
	if err != nil {
		return err
	}
	containerID := resp.ID
	defer func() {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), teardownTimeout)
		defer cancel()
		if rmErr := cm.RemoveContainerForce(rmCtx, containerID); rmErr != nil {
			logger.Printf("[Docker] Error removing shell container: %v", rmErr)
		}
	}()

	status("Copying workspace...")
	var b bytes.Buffer
	if err := archive.CreateFSTar(wd, &b); err != nil {
		return err
	}
	if err := cm.CopyToContainer(ctx, containerID, job.Workdir, &b, container.CopyToContainerOptions{}); err != nil {
		return err
	}

	hijack, err := cm.AttachLogger(ctx, containerID, container.AttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return err
	}
	defer hijack.Close()

	if err := cm.StartContainer(ctx, containerID, container.StartOptions{}); err != nil {
		return err
	}

	status("Entering %s (job %q) — type 'exit' to leave.", job.Image, job.Name)

	// Raw terminal for the duration of the session; restored before teardown
	// messages print.
	rawState, err := term.SetRawTerminal(stdinFd)
	if err != nil {
		return fmt.Errorf("set raw terminal: %w", err)
	}
	restore := func() { _ = term.RestoreTerminal(stdinFd, rawState) }
	defer restore()

	// Keep the container's TTY sized to the local terminal. Polling instead of
	// SIGWINCH keeps this portable (no unix-only signal handling).
	resizeCtx, stopResize := context.WithCancel(ctx)
	defer stopResize()
	go trackTerminalSize(resizeCtx, cli, containerID, stdinFd)

	go func() {
		_, _ = io.Copy(hijack.Conn, os.Stdin)
		_ = hijack.CloseWrite()
	}()

	// TTY mode: the stream is not multiplexed; copy until the shell exits.
	_, _ = io.Copy(os.Stdout, hijack.Reader)

	restore()
	status("\nShell session ended.")
	return nil
}

// trackTerminalSize keeps the container TTY in sync with the local terminal,
// polling for size changes twice a second.
func trackTerminalSize(ctx context.Context, cli *client.Client, containerID string, fd uintptr) {
	var lastH, lastW uint16
	for {
		if ws, err := term.GetWinsize(fd); err == nil && (ws.Height != lastH || ws.Width != lastW) {
			lastH, lastW = ws.Height, ws.Width
			_ = cli.ContainerResize(ctx, containerID, container.ResizeOptions{
				Height: uint(ws.Height), Width: uint(ws.Width),
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
