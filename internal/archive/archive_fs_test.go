package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateFSTarPreservesPortableDirectoryStructure(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"root.txt":                        "root file",
		"src/main.go":                     "package main\n",
		"src/nested/file with spaces.txt": "nested file\n",
	}
	for name, content := range files {
		filename := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(src, "empty", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := CreateFSTar(src, &buf); err != nil {
		t.Fatal(err)
	}

	// Docker extracts these names using Linux path rules, even when the
	// workspace was archived on Windows. Check the actual tar entries and
	// contents, including directories with no files to recreate them.
	wantTypes := map[string]byte{
		"root.txt":                        tar.TypeReg,
		"src":                             tar.TypeDir,
		"src/main.go":                     tar.TypeReg,
		"src/nested":                      tar.TypeDir,
		"src/nested/file with spaces.txt": tar.TypeReg,
		"empty":                           tar.TypeDir,
		"empty/nested":                    tar.TypeDir,
	}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		wantType, ok := wantTypes[name]
		if !ok {
			t.Errorf("unexpected archive entry %q; expected slash-separated relative paths", hdr.Name)
			continue
		}
		if hdr.Typeflag != wantType {
			t.Errorf("entry %q type = %d, want %d", name, hdr.Typeflag, wantType)
		}
		if hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != files[name] {
				t.Errorf("entry %q content = %q, want %q", name, content, files[name])
			}
		}
		delete(wantTypes, name)
	}
	for name := range wantTypes {
		t.Errorf("missing archive entry %q", name)
	}
}
