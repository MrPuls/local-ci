package docker

import (
	"bytes"
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"local-ci/cmd/archive"
	"local-ci/cmd/config"
	"os"
	"strings"
)

func ExecuteConfigPipeline(cfg config.Config, ctx context.Context) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {
			panic(err)
		}
	}(cli)

	reader, err := cli.ImagePull(ctx, cfg.Blocks["Test"].Image, image.PullOptions{})
	if err != nil {
		panic(err)
	}
	_, errCp := io.Copy(os.Stdout, reader)
	if errCp != nil {
		panic(errCp)
	}

	shellCmd := strings.Join(cfg.Blocks["Test"].Script, "&&")

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      cfg.Blocks["Test"].Image,
		WorkingDir: "/app",
		Cmd:        []string{"/bin/sh", "-c", shellCmd},
	}, nil, nil, nil, "")
	if err != nil {
		panic(err)
	}

	var b bytes.Buffer
	fsErr := archive.CreateFSTar(".", &b)
	if fsErr != nil {
		return
	}

	errCpCtr := cli.CopyToContainer(ctx, resp.ID, "/app", &b, container.CopyToContainerOptions{})
	if errCpCtr != nil {
		panic(errCpCtr)
	}

	logs, err := cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		panic(err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	}
	defer logs.Close()

	_, err = io.Copy(os.Stdout, logs.Reader)
	if err != nil && err != io.EOF {
		panic(err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}

	delCntErr := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
	if delCntErr != nil {
		panic(delCntErr)
	}
}
