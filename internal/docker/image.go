package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	localconfig "github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type ImageManager struct {
	client     *client.Client
	adapter    ConfigAdapter
	logger     *log.Logger
	registries localconfig.Registries
}

func NewImageManager(cli *client.Client, adapter ConfigAdapter, logger *log.Logger, registries localconfig.Registries) *ImageManager {
	return &ImageManager{
		client:     cli,
		adapter:    adapter,
		logger:     logger,
		registries: registries,
	}
}

// EnsureImage reuses a local image unless the config requests a fresh pull.
// Only a missing image triggers a pull; daemon and inspection errors are fatal.
func (i *ImageManager) EnsureImage(ctx context.Context, cfg localconfig.ImageConfig, out io.Writer) error {
	if !cfg.ForcePull {
		if _, err := i.client.ImageInspect(ctx, cfg.Name); err == nil {
			i.logger.Printf("[Docker] Using local image %q", cfg.Name)
			return nil
		} else if !client.IsErrNotFound(err) {
			return fmt.Errorf("inspect image %q: %w", cfg.Name, err)
		}
	}

	reader, err := i.PullImage(ctx, cfg.Name, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %q: %w", cfg.Name, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			i.logger.Printf("[Docker] Error closing image pull reader: %v", err)
		}
	}()

	// Docker may report a pull failure in the JSON stream after sending HTTP
	// 200. Treat it as a failure even when an older local image is available.
	decoder := json.NewDecoder(io.TeeReader(reader, out))
	for {
		var progress struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&progress); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("read image pull %q: %w", cfg.Name, err)
		}
		if progress.ErrorDetail.Message != "" {
			return fmt.Errorf("pull image %q: %s", cfg.Name, progress.ErrorDetail.Message)
		}
		if progress.Error != "" {
			return fmt.Errorf("pull image %q: %s", cfg.Name, progress.Error)
		}
	}
	i.logger.Printf("[Docker] Image %q is pulled", cfg.Name)
	return nil
}

func (i *ImageManager) PullImage(ctx context.Context, image string, options image.PullOptions) (io.ReadCloser, error) {
	i.logger.Printf("[Docker] Pulling image %q...", image)
	if options.RegistryAuth == "" {
		auth, err := i.adapter.ToRegistryAuth(image, i.registries)
		if err != nil {
			return nil, err
		}
		options.RegistryAuth = auth
	}
	return i.client.ImagePull(ctx, image, options)
}
