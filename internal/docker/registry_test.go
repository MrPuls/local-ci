package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"go.yaml.in/yaml/v4"
)

func savedDockerLogins(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	auth := base64.StdEncoding.EncodeToString([]byte("saved-user:saved-password"))
	data, err := json.Marshal(map[string]any{"auths": map[string]any{
		"registry.example.com":        map[string]string{"auth": auth},
		"https://index.docker.io/v1/": map[string]string{"auth": auth},
		"token.example.com":           map[string]string{"identitytoken": "saved-identity-token"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRegistryAuthSelection(t *testing.T) {
	savedDockerLogins(t)
	t.Setenv("LOCAL_CI_REGISTRY_USER", "configured-user")
	t.Setenv("LOCAL_CI_REGISTRY_TOKEN", "configured-token")
	configured := config.RegistryConfig{UsernameEnv: "LOCAL_CI_REGISTRY_USER", PasswordEnv: "LOCAL_CI_REGISTRY_TOKEN"}
	for _, tt := range []struct {
		name, image, user, password, identity string
		registries                            config.Registries
	}{
		{"explicit overrides login", "registry.example.com/team/app:latest", "configured-user", "configured-token", "", config.Registries{"registry.example.com": configured}},
		{"saved login", "registry.example.com/team/app:latest", "saved-user", "saved-password", "", nil},
		{"saved Docker Hub login", "team/private:latest", "saved-user", "saved-password", "", nil},
		{"explicit Docker Hub", "ubuntu", "configured-user", "configured-token", "", config.Registries{"docker.io": configured}},
		{"identity token", "token.example.com/team/app:v1", "", "", "saved-identity-token", nil},
		{"no cross-registry credentials", "public.example.com/team/app:v1", "", "", "", config.Registries{"registry.example.com": configured}},
		{"exact port", "registry.example.com:5000/team/app:v1", "configured-user", "configured-token", "", config.Registries{"registry.example.com:5000": configured}},
		{"different port", "registry.example.com:5001/team/app:v1", "", "", "", config.Registries{"registry.example.com:5000": configured}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Registries:     tt.registries,
				RemoteProvider: &config.RemoteProvider{Url: "https://registry.example.com"},
			}
			adapter := NewConfigAdapter(cfg, log.New(io.Discard, "", 0))
			encoded, err := adapter.ToRegistryAuth(tt.image, nil)
			if err != nil {
				t.Fatal(err)
			}
			auth, err := registry.DecodeAuthConfig(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if auth.Username != tt.user || auth.Password != tt.password || auth.IdentityToken != tt.identity {
				t.Error("authentication does not match the credentials for the image's registry")
			}
		})
	}
}

func TestRegistryAuthMissingEnvironmentDoesNotFallBack(t *testing.T) {
	savedDockerLogins(t)
	t.Setenv("LOCAL_CI_REGISTRY_TOKEN", "")
	adapter := NewConfigAdapter(&config.Config{Registries: config.Registries{
		"registry.example.com": {Username: "ci-user", PasswordEnv: "LOCAL_CI_REGISTRY_TOKEN"},
	}}, log.New(io.Discard, "", 0))
	if _, err := adapter.ToRegistryAuth("registry.example.com/team/app:v1", nil); err == nil || !strings.Contains(err.Error(), "LOCAL_CI_REGISTRY_TOKEN") {
		t.Fatalf("expected missing environment variable error, got %v", err)
	}
}

func TestRegistryAuthDoesNotExposeMalformedSavedCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"auths":{"registry.example.com":{"auth":"secret-not-base64"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	adapter := NewConfigAdapter(&config.Config{}, log.New(io.Discard, "", 0))
	_, err := adapter.ToRegistryAuth("registry.example.com/team/app:v1", nil)
	if err == nil || strings.Contains(err.Error(), "secret-not-base64") {
		t.Fatalf("expected a sanitized configuration error, got %v", err)
	}
}

func TestEnsurePrivateImageAuthentication(t *testing.T) {
	t.Setenv("LOCAL_CI_PRIVATE_TOKEN", "private-test-token")
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	adapter := NewConfigAdapter(&config.Config{Registries: config.Registries{
		"registry.example.com": {Username: "ci-user", PasswordEnv: "LOCAL_CI_PRIVATE_TOKEN"},
	}}, logger)
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%v", local), func(t *testing.T) {
			pulls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/json") {
					if !local {
						w.WriteHeader(http.StatusNotFound)
					}
					fmt.Fprint(w, `{}`)
					return
				}
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/images/create") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				pulls++
				auth, err := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
				if err != nil || auth.Username != "ci-user" || auth.Password != "private-test-token" || auth.ServerAddress != "registry.example.com" {
					t.Error("pull request is missing the configured registry credentials")
				}
				fmt.Fprint(w, "{\"status\":\"Done\"}\n")
			}))
			defer srv.Close()
			cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.51"), client.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			im := NewImageManager(cli, adapter, logger, nil)
			if local {
				// Reusing a local image must not require registry credentials.
				t.Setenv("LOCAL_CI_PRIVATE_TOKEN", "")
			}
			image := config.ImageConfig{Name: "registry.example.com/team/app:v1"}
			if err := im.EnsureImage(context.Background(), image, io.Discard); err != nil {
				t.Fatal(err)
			}
			if local && pulls != 0 || !local && pulls != 1 {
				t.Errorf("unexpected pull count %d for local=%v", pulls, local)
			}
			// A forced pull must authenticate even when the image is local.
			t.Setenv("LOCAL_CI_PRIVATE_TOKEN", "private-test-token")
			image.ForcePull = true
			if err := im.EnsureImage(context.Background(), image, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
	if strings.Contains(logs.String(), "private-test-token") {
		t.Fatal("registry password leaked into logs")
	}
}

func TestJobRegistryAuthPrecedence(t *testing.T) {
	savedDockerLogins(t)
	logger := log.New(io.Discard, "", 0)
	pipeline := config.Registries{
		"registry.example.com": {Username: "pipeline-user", Password: "pipeline-password"},
	}
	adapter := NewConfigAdapter(&config.Config{Registries: pipeline}, logger)
	for _, tt := range []struct {
		name, image, wantUser string
		registries            config.Registries
	}{
		{"job overrides pipeline", "registry.example.com/team/app:v1", "job-user", config.Registries{
			"registry.example.com": {Username: "job-user", Password: "job-password"},
		}},
		{"other host keeps pipeline default", "registry.example.com/team/app:v1", "pipeline-user", config.Registries{
			"other.example.com": {Username: "other-user", Password: "other-password"},
		}},
		{"sibling keeps pipeline default", "registry.example.com/team/app:v1", "pipeline-user", nil},
		{"saved login fallback", "ubuntu", "saved-user", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := adapter.ToRegistryAuth(tt.image, tt.registries)
			if err != nil {
				t.Fatal(err)
			}
			auth, err := registry.DecodeAuthConfig(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if auth.Username != tt.wantUser {
				t.Error("did not select the credentials with the correct precedence")
			}
		})
	}
	t.Setenv("LOCAL_CI_JOB_TOKEN", "")
	_, err := adapter.ToRegistryAuth("registry.example.com/team/app:v1", config.Registries{
		"registry.example.com": {Username: "job-user", PasswordEnv: "LOCAL_CI_JOB_TOKEN"},
	})
	if err == nil || !strings.Contains(err.Error(), "LOCAL_CI_JOB_TOKEN") {
		t.Fatalf("missing job credentials must fail instead of using pipeline credentials, got %v", err)
	}
}

func TestExecutorUsesJobCredentialsBeforeContainerCreation(t *testing.T) {
	for _, service := range []bool{false, true} {
		t.Run(fmt.Sprintf("service=%v", service), func(t *testing.T) {
			var cfg config.Config
			input := `
registries:
  registry.example.com:
    username: pipeline-user
    password: pipeline-password
stages: [build]
Build:
  stage: build
  registries:
    registry.example.com:
      username: job-user
      password: job-password
  image: registry.example.com/team/build:latest
  script: [echo hi]
`
			wantImage := "registry.example.com/team/build"
			if service {
				input += "  services:\n    - registry.example.com/team/service:latest\n"
				wantImage = "registry.example.com/team/service"
			}
			if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
				t.Fatal(err)
			}
			if err := config.ValidateConfig(&cfg); err != nil {
				t.Fatal(err)
			}
			pulls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/networks/create"):
					fmt.Fprint(w, `{"Id":"test-network"}`)
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/images/"):
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"message":"image missing"}`)
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images/create"):
					pulls++
					if r.URL.Query().Get("fromImage") != wantImage {
						t.Error("executor pulled the wrong image")
					}
					auth, err := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
					if err != nil || auth.Username != "job-user" || auth.Password != "job-password" {
						t.Error("executor did not use the job's registry credentials")
					}
					fmt.Fprint(w, "{\"status\":\"Done\"}\n")
				case r.Method == http.MethodDelete:
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create"):
					// Stop here: the pull/authentication path is what this test verifies.
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"message":"stop after pull"}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer srv.Close()
			cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.51"), client.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			logger := log.New(io.Discard, "", 0)
			executor := NewDockerExecutor(cli, NewConfigAdapter(&cfg, logger), logger)
			defer executor.Close()
			err = executor.Execute(context.Background(), cfg.Jobs[0], io.Discard)
			if err == nil || !strings.Contains(err.Error(), "stop after pull") || pulls != 1 {
				t.Fatalf("executor error = %v, pulls = %d; want stop after one authenticated pull", err, pulls)
			}
		})
	}
}
