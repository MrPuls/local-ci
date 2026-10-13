package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
	"github.com/docker/docker/client"
)

func TestEnsureImage(t *testing.T) {
	const progress = "{\"status\":\"Pulling\"}\n{\"status\":\"Done\"}\n"
	for _, tt := range []struct {
		name          string
		forcePull     bool
		inspectStatus int
		pullStatus    int
		pullBody      string
		wantCalls     []string
		wantErr       string
	}{
		{"reuse local image", false, 200, 200, progress, []string{"inspect"}, ""},
		{"pull missing image", false, 404, 200, progress, []string{"inspect", "pull"}, ""},
		{"force pull local image", true, 200, 200, progress, []string{"pull"}, ""},
		{"force pull missing image", true, 404, 200, progress, []string{"pull"}, ""},
		{"daemon failure", false, 500, 200, progress, []string{"inspect"}, "inspect image"},
		{"inspection forbidden", false, 403, 200, progress, []string{"inspect"}, "inspect image"},
		{"missing image pull failure", false, 404, 500, `{"message":"registry unavailable"}`, []string{"inspect", "pull"}, "registry unavailable"},
		{"forced pull failure", true, 200, 500, `{"message":"registry unavailable"}`, []string{"pull"}, "registry unavailable"},
		{"stream error", true, 200, 200, progress + `{"error":"pull denied"}`, []string{"pull"}, "pull denied"},
		{"stream error detail", true, 200, 200, `{"errorDetail":{"message":"manifest missing"}}`, []string{"pull"}, "manifest missing"},
		{"broken stream", false, 404, 200, `{"status":`, []string{"inspect", "pull"}, "read image pull"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/images/ubuntu/json"):
					calls = append(calls, "inspect")
					w.WriteHeader(tt.inspectStatus)
					if tt.inspectStatus == 200 {
						fmt.Fprint(w, `{"Id":"sha256:local"}`)
					} else {
						fmt.Fprint(w, `{"message":"inspect failed"}`)
					}
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images/create"):
					calls = append(calls, "pull")
					if got := r.URL.Query().Get("fromImage"); got != "docker.io/library/ubuntu" {
						t.Errorf("pull image = %q, want docker.io/library/ubuntu", got)
					}
					w.WriteHeader(tt.pullStatus)
					fmt.Fprint(w, tt.pullBody)
				default:
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
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
			im := NewImageManager(cli, NewConfigAdapter(&config.Config{}, logger), logger)
			var out bytes.Buffer
			err = im.EnsureImage(context.Background(), config.ImageConfig{Name: "ubuntu", ForcePull: tt.forcePull}, &out)
			if tt.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("Docker calls = %v, want %v", calls, tt.wantCalls)
			}
			if tt.wantErr == "" {
				wantOutput := ""
				if calls[len(calls)-1] == "pull" {
					wantOutput = progress
				}
				if out.String() != wantOutput {
					t.Errorf("pull output = %q, want %q", out.String(), wantOutput)
				}
			}
		})
	}
}

func TestEnsureImageInspectsExactReference(t *testing.T) {
	for _, ref := range []string{"ubuntu:24.04", "registry.example.com/team/app:v2", "ubuntu@sha256:" + strings.Repeat("a", 64)} {
		t.Run(ref, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/images/"+ref+"/json") {
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"Id":"sha256:local"}`)
			}))
			defer srv.Close()
			cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.51"), client.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			logger := log.New(io.Discard, "", 0)
			im := NewImageManager(cli, NewConfigAdapter(&config.Config{}, logger), logger)
			if err := im.EnsureImage(context.Background(), config.ImageConfig{Name: ref}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
