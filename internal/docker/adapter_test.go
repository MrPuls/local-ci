package docker

import (
	"io"
	"log"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/mount"
)

func TestCacheMountsUseContainerPaths(t *testing.T) {
	adapter := NewConfigAdapter(&config.Config{}, log.New(io.Discard, "", 0))
	for _, tt := range []struct {
		name, workdir, cachePath, wantTarget, wantSource string
	}{
		{"root workdir", "/", ".cache", "/.cache", "test-deps-.cache"},
		{"absolute workdir", "/app", ".cache", "/app/.cache", "test-deps-app-.cache"},
		{"relative workdir", "app", ".cache", "/app/.cache", "test-deps-app-.cache"},
		{"nested cache", "/app/", "build/cache", "/app/build/cache", "test-deps-app-build-cache"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			host := adapter.ToHostConfig(config.JobConfig{
				Name: "test", Workdir: tt.workdir,
				Cache: &config.CacheConfig{Key: "deps", Paths: []string{tt.cachePath}},
			})
			if len(host.Mounts) != 1 {
				t.Fatalf("got %d mounts, want 1", len(host.Mounts))
			}
			got := host.Mounts[0]
			if got.Type != mount.TypeVolume || got.Target != tt.wantTarget || got.Source != tt.wantSource {
				t.Errorf("mount = %+v, want volume %q at %q", got, tt.wantSource, tt.wantTarget)
			}
		})
	}
}
