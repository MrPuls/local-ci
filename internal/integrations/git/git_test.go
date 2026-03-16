package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrPuls/local-ci/internal/integrations/fs"
)

func TestRemoteString(t *testing.T) {
	remoteHttp := "https://gitlab.com/MrPuls/test-proj.git"
	httpSplit := strings.Split(remoteHttp, "/")
	httpSlug := httpSplit[len(httpSplit)-1]
	proj_name := strings.TrimSuffix(httpSlug, ".git")
	if proj_name != "test-proj" {
		t.Errorf("expected test-proj, got %s", proj_name)
	}
}

func TestClone(t *testing.T) {
	remoteHttp := "https://gitlab.com/MrPuls/test-proj.git"
	dir, err := fs.GetDefaultDir()
	if err != nil {
		t.Errorf("expected nil, got %s", err)
	}
	err = Clone(dir, remoteHttp)
	if err != nil {
		t.Errorf("expected nil, got %s", err)
	}
	dirExists, err := fs.IsDirExists(filepath.Join(dir, "test-proj"))
	if err != nil {
		t.Errorf("expected nil, got %s", err)
	}
	if !dirExists {
		t.Errorf("Repository was not cloned properly")
	}

}
