package git

import (
	"strings"
	"testing"
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
