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

func ExecuteConfigPipeline(wd string, yamlConf config.Config) {
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

	for block, cfg := range yamlConf.Blocks {
		workdir := cfg.Workdir
		if workdir == "" {
			workdir = "/"
		}

		reader, err := cli.ImagePull(ctx, cfg.Image, image.PullOptions{})
		fmt.Println("Image is pulled")
		if err != nil {
			panic(err)
		}
		_, errCp := io.Copy(os.Stdout, reader)
		if errCp != nil {
			panic(errCp)
		}

		shellCmd := strings.Join(cfg.Script, "&&")

		var envVars []string
		// append global vars, skip if var is present on block level
		for k, v := range yamlConf.GlobalVariables {
			if _, ok := cfg.Variables[k]; ok {
				continue
			}
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}
		// append block level vars
		for k, v := range cfg.Variables {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Println("Trying to create a container!")
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:      cfg.Image,
			WorkingDir: workdir,
			Cmd:        []string{"/bin/sh", "-c", shellCmd},
			Env:        envVars,
		}, nil, nil, nil, block)
		if err != nil {
			panic(err)
		}

		var b bytes.Buffer
		fmt.Println("Trying to create a fs tar!")
		fsErr := archive.CreateFSTar(wd, &b)
		if fsErr != nil {
			panic(fsErr)
		}

		fmt.Println("Trying to copy files to container!")
		errCpCtr := cli.CopyToContainer(ctx, resp.ID, workdir, &b, container.CopyToContainerOptions{})
		if errCpCtr != nil {
			panic(errCpCtr)
		}

		logs, err := cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
		if err != nil {
			panic(err)
		}

		fmt.Println("Trying to start a container!")
		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			panic(err)
		}

		fmt.Println("Copying files to container!")
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

		fmt.Println("Job is done, removing container...")
		delCntErr := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
		if delCntErr != nil {
			panic(delCntErr)
		}

		logs.Close()

		fmt.Println("All done!")
	}
}
