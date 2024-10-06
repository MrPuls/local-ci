package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"local-ci/cmd/config"
	"strings"
)

type Docker struct {
	client *client.Client
	ctx    context.Context
	config *config.Config
}

func (d *Docker) CreateClient(ctx context.Context) {
	d.ctx = ctx
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
	d.client = cli
}

func (d *Docker) PullImage(pullOptions image.PullOptions) (io.ReadCloser, error) {
	cli := d.client
	reader, err := cli.ImagePull(d.ctx, d.config.Blocks["Test"].Image, pullOptions)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (d *Docker) CreateContainer(config config.Config) (container.CreateResponse, error) {
	cli := d.client
	shellCmd := strings.Join(config.Blocks["Test"].Script, "&&")

	resp, err := cli.ContainerCreate(d.ctx, &container.Config{
		Image:      config.Blocks["Test"].Image,
		WorkingDir: "/app",
		Cmd:        []string{"/bin/sh", "-c", shellCmd},
	}, nil, nil, nil, "")
	if err != nil {
		return resp, err
	}
	return resp, nil
}
