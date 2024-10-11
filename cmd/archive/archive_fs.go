package archive

import (
	"archive/tar"
	"bytes"
	"os"
)

func CreateFSTar(src string, dest *bytes.Buffer) error {
	tw := tar.NewWriter(dest)
	fileSystem := os.DirFS(src)
	err := tw.AddFS(fileSystem)
	if err != nil {
		return err
	}
	errClose := tw.Close()
	if errClose != nil {
		return err
	}
	return nil
}
