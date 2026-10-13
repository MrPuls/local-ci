package docker

import (
	"fmt"
	"os"
	"strings"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/distribution/reference"
	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/registry"
)

// ToRegistryAuth sends credentials only to the registry named by the image.
// Job credentials override pipeline credentials, which override Docker's saved
// login for that exact host. The shared pipeline configuration is never changed.
func (a *configAdapter) ToRegistryAuth(imageName string, jobRegistries config.Registries) (string, error) {
	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", fmt.Errorf("invalid image reference: %w", err)
	}
	host := strings.ToLower(reference.Domain(ref))
	credentials, exists := jobRegistries[host]
	if !exists {
		credentials, exists = a.config.Registries[host]
	}
	if exists {
		username, password, err := credentials.Resolve()
		if err != nil {
			return "", fmt.Errorf("registry %q: %w", host, err)
		}
		return registry.EncodeAuthConfig(registry.AuthConfig{
			ServerAddress: host,
			Username:      username,
			Password:      password,
		})
	}

	// Docker Hub uses this legacy key in config.json and credential helpers.
	if host == "docker.io" {
		host = "https://index.docker.io/v1/"
	}
	configFile, err := dockerconfig.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return "", fmt.Errorf("cannot load Docker login configuration for registry %q", host)
	}
	auth, err := configFile.GetAuthConfig(host)
	if err != nil {
		// Credential helper errors may include sensitive helper output.
		return "", fmt.Errorf("cannot read Docker login credentials for registry %q", host)
	}
	return registry.EncodeAuthConfig(registry.AuthConfig{
		Username:      auth.Username,
		Password:      auth.Password,
		Auth:          auth.Auth,
		ServerAddress: auth.ServerAddress,
		IdentityToken: auth.IdentityToken,
		RegistryToken: auth.RegistryToken,
	})
}
