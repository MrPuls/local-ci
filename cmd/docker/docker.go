package docker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/archive"
	"github.com/MrPuls/local-ci/cmd/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"os"
	"strings"
	"time"
)

func ExecuteConfigPipeline(cfg config.StepConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	workdir := cfg.Workdir
	if workdir == "" {
		workdir = "/"
	}
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

	reader, err := cli.ImagePull(ctx, cfg.Image, image.PullOptions{})
	if err != nil {
		panic(err)
	}
	_, errCp := io.Copy(os.Stdout, reader)
	if errCp != nil {
		panic(errCp)
	}

	shellCmd := strings.Join(cfg.Script, "&&")

	var envVars []string
	for k, v := range cfg.Variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      cfg.Image,
		WorkingDir: workdir,
		Cmd:        []string{"/bin/sh", "-c", shellCmd},
		Env:        envVars,
	}, nil, nil, nil, "")
	if err != nil {
		panic(err)
	}
	// TODO: must only copy files from the project dir. Also have to somehow determine that this is the project dir
	// 	can be done through convention that the config should be in ./local dir and
	//		then copy everything in the one dir above
	var b bytes.Buffer
	fsErr := archive.CreateFSTar(".", &b)
	if fsErr != nil {
		return
	}

	errCpCtr := cli.CopyToContainer(ctx, resp.ID, workdir, &b, container.CopyToContainerOptions{})
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
