package fs

import (
	"fmt"
	"os"
	"testing"
)

func TestChDir(t *testing.T) {
	dir, err := MakeDefaultDir()
	if err != nil {
		t.Fatalf("failed to get default dir: %v", err)
	}

	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	wd, err := os.Getwd()
	fmt.Printf("current dir: %s\n", wd)
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if wd != dir {
		t.Fatalf("expected dir %s, got %s", dir, wd)
	}

}
