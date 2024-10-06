package main

import (
	"archive/tar"
	"bytes"
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"local-ci/cmd/config"
	"os"
	"strings"
	"time"
)

func main() {
	yamlConf := config.Config{}
	err := yamlConf.GetConfig()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
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

	reader, err := cli.ImagePull(ctx, yamlConf.Blocks["Test"].Image, image.PullOptions{})
	if err != nil {
		panic(err)
	}
	_, errCp := io.Copy(os.Stdout, reader)
	if errCp != nil {
		panic(errCp)
	}

	shellCmd := strings.Join(yamlConf.Blocks["Test"].Script, "&&")

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      yamlConf.Blocks["Test"].Image,
		WorkingDir: "/app",
		Cmd:        []string{"/bin/sh", "-c", shellCmd},
	}, nil, nil, nil, "")
	if err != nil {
		panic(err)
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	fileSystem := os.DirFS(".")
	errTW := tw.AddFS(fileSystem)
	if errTW != nil {
		return
	}
	errClose := tw.Close()
	if errClose != nil {
		return
	}

	errCpCntr := cli.CopyToContainer(ctx, resp.ID, "/app", &b, container.CopyToContainerOptions{})
	if errCpCntr != nil {
		panic(errCpCntr)
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
