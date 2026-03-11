package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type ImageManager struct {
	client  *client.Client
	adapter ConfigAdapter
}

func NewImageManager(cli *client.Client, adapter ConfigAdapter) *ImageManager {
	return &ImageManager{
		client:  cli,
		adapter: adapter,
	}
}

func (i *ImageManager) PullImage(ctx context.Context, image string, options image.PullOptions) (io.ReadCloser, error) {
	log.Printf("Pulling image %q...", image)
	hostname := i.adapter.ToImageHostConfig(image)
	if hostname != "" {
		configFile := config.LoadDefaultConfigFile(os.Stderr)
		authConf, err := configFile.GetAuthConfig(hostname)
		if err != nil {
			return nil, err
		}
		encodeAuth, err := json.Marshal(authConf)
		if err != nil {
			return nil, err
		}
		authStr := base64.URLEncoding.EncodeToString(encodeAuth)
		options.RegistryAuth = authStr
	}
	return i.client.ImagePull(ctx, image, options)
}
