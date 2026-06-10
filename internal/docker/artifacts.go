package docker

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
)

// artifactStore holds the artifacts collected from finished jobs for the
// duration of one run, as tar files in a temp dir, ready to be streamed into
// later job containers. Entry names are rewritten at collection time so each
// tar extracts at the artifact's original path relative to the workdir
// ("dist/sub/…", not "sub/…").
type artifactStore struct {
	mu    sync.Mutex
	dir   string
	files []artifactFile
}

type artifactFile struct {
	path string // tar file on disk
	from string // producing job, for diagnostics
}

// collect stores one artifact tar (as returned by CopyFromContainer, rooted at
// the path's base name), prepending prefix to every entry. prefix "." keeps
// entries unchanged.
func (s *artifactStore) collect(from string, r io.Reader, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		dir, err := os.MkdirTemp("", "local-ci-artifacts-")
		if err != nil {
			return err
		}
		s.dir = dir
	}

	f, err := os.CreateTemp(s.dir, "artifact-*.tar")
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(r)
	tw := tar.NewWriter(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read artifact tar: %w", err)
		}
		if prefix != "." && prefix != "" {
			hdr.Name = path.Join(prefix, hdr.Name)
			if hdr.Typeflag == tar.TypeLink { // hard links point inside the same tar
				hdr.Linkname = path.Join(prefix, hdr.Linkname)
			}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return fmt.Errorf("copy artifact entry %s: %w", hdr.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	s.files = append(s.files, artifactFile{path: f.Name(), from: from})
	return nil
}

// snapshot returns the artifacts collected so far, in collection order.
func (s *artifactStore) snapshot() []artifactFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]artifactFile, len(s.files))
	copy(out, s.files)
	return out
}

func (s *artifactStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return nil
	}
	dir := s.dir
	s.dir, s.files = "", nil
	return os.RemoveAll(dir)
}
