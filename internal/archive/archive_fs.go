package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func getIgnorePatterns(src string) ([]string, error) {
	// Read .gitignore if it exists
	var ignorePatterns []string // default patterns we always want to ignore
	gitignorePath := filepath.Join(src, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		content, err := os.ReadFile(gitignorePath)
		if err == nil {
			// Split by newlines and add non-empty lines
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					ignorePatterns = append(ignorePatterns, line)
				}
			}
		}
	}
	return ignorePatterns, nil
}

func CreateFSTar(src string, dest *bytes.Buffer) error {
	ignorePatterns, err := getIgnorePatterns(src)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(dest)
	defer func(tw *tar.Writer) {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %v", err)
		}
	}(tw)

	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip if matches any ignore pattern
		for _, pattern := range ignorePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			defer func(file *os.File) {
				err := file.Close()
				if err != nil {
					log.Printf("Error closing tar file: %v", err)
				}
			}(file)

			_, err = io.Copy(tw, file)
			return err
		}

		return nil
	})
}
