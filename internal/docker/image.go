package docker

import (
	"context"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"log"
)

type ImageManager struct {
	client *client.Client
}

func NewImageManager(cli *client.Client) *ImageManager {
	return &ImageManager{
		client: cli,
	}
}

func (i *ImageManager) PullImage(ctx context.Context, image string, options image.PullOptions) (io.ReadCloser, error) {
	log.Printf("Pulling image %q...", image)
	return i.client.ImagePull(ctx, image, options)
}
