package docker

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/client"
)

// Status reports whether a local container engine is reachable and which one.
type Status struct {
	Ready    bool   `json:"ready"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

// Probe checks the local container engine: it connects using the standard
// environment (DOCKER_HOST etc.), asks the daemon for info, and identifies the
// provider (Docker, Docker Desktop, OrbStack, …). Ready is false — with the best
// guess at a provider name — when no daemon answers within a short timeout.
func Probe(ctx context.Context) Status {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return Status{Provider: "Docker"}
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	info, err := cli.Info(ctx)
	if err != nil {
		return Status{Provider: "Docker"}
	}
	return Status{
		Ready:    true,
		Provider: providerName(info.OperatingSystem, info.Name),
		Version:  info.ServerVersion,
	}
}

// providerName maps the daemon's self-reported OS/name to a friendly engine
// label. OrbStack and Docker Desktop both expose a Docker-compatible API but
// identify themselves in `docker info`.
func providerName(osName, name string) string {
	hay := strings.ToLower(osName + " " + name)
	switch {
	case strings.Contains(hay, "orbstack"):
		return "OrbStack"
	case strings.Contains(hay, "docker desktop"):
		return "Docker Desktop"
	case strings.Contains(hay, "rancher"):
		return "Rancher Desktop"
	case strings.Contains(hay, "podman"):
		return "Podman"
	default:
		return "Docker"
	}
}
