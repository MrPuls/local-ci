package git

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MrPuls/local-ci/internal/integrations/fs"
)

func Clone(dir string, repoURL string) error {
	log.Println("Cloning repo...")
	cmd := exec.Command("git", "clone", repoURL)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}
	return nil
}

func Update(dir string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Found existing repo, updating...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update repo: %w", err)
	}
	return nil
}

func GetRepoName(repoURL string) (string, error) {
	httpSplit := strings.Split(repoURL, "/")
	proj_name := strings.TrimSuffix(httpSplit[len(httpSplit)-1], ".git")
	return proj_name, nil
}

func SetupLocal(remote string) error {
	if remote == "" {
		return nil
	}
	repo_name, err := GetRepoName(remote)
	log.Printf("repo_name: %s", repo_name)
	if err != nil {
		return fmt.Errorf("failed to get repo name: %w", err)
	}
	dir, err := fs.MakeDefaultDir()
	if err != nil {
		return fmt.Errorf("failed to create git dir: %w", err)
	}

	repo_path := filepath.Join(dir, repo_name)

	isRepoExist, err := fs.IsDirExists(repo_path)
	if err != nil {
		return fmt.Errorf("failed to check if repo exists: %w", err)
	}
	if isRepoExist {
		err := Update(repo_path)
		if err != nil {
			return fmt.Errorf("failed to pull repo: %w", err)
		}
	} else {
		err := Clone(dir, remote)
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
	}
	log.Printf("Changing dir to: %s", repo_path)
	err = os.Chdir(repo_path)
	if err != nil {
		return fmt.Errorf("failed to change dir to %s: %w", repo_path, err)
	}
	return nil
}
