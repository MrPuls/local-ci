package fs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func MakeDefaultDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(base, "local-ci")
	fmt.Println(dir)
	_, err := os.Stat(dir)
	if err == nil {
		log.Println("Local dir already exists, skipping creation...")
		return dir, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dir: %w", err)
	}
	return dir, nil
}

func IsDirExists(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if dir exists: %w", err)
}

func GetDefaultDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "local-ci"), nil
}
